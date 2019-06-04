package tiltfile

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockerfile"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
)

const fastBuildDeprecationWarning = "FastBuild (`fast_build`; `add_fast_build`; " +
	"`docker_build(...).add(...)`, etc.) will be deprecated soon; you can use Live " +
	"Update instead! See https://docs.tilt.dev/live_update_tutorial.html for more " +
	"information. If Live Update doesn't fit your use case, let us know."

type dockerImage struct {
	baseDockerfilePath localPath
	baseDockerfile     dockerfile.Dockerfile
	configurationRef   container.RefSelector
	deploymentRef      reference.Named
	syncs              []sync
	runs               []model.Run
	entrypoint         string
	cachePaths         []string
	hotReload          bool
	matchInEnvVars     bool
	ignores            []string

	dbDockerfilePath localPath
	dbDockerfile     dockerfile.Dockerfile
	dbBuildPath      localPath
	dbBuildArgs      model.DockerBuildArgs

	customCommand string
	customDeps    []string
	customTag     string

	// Whether this has been matched up yet to a deploy resource.
	matched bool

	dependencyIDs []model.TargetID
	disablePush   bool

	liveUpdate model.LiveUpdate
}

func (d *dockerImage) ID() model.TargetID {
	return model.ImageID(d.configurationRef)
}

type dockerImageBuildType int

const (
	UnknownBuild = iota
	DockerBuild
	FastBuild
	CustomBuild
)

func (d *dockerImage) Type() dockerImageBuildType {
	if !d.dbBuildPath.Empty() {
		return DockerBuild
	}

	if !d.baseDockerfilePath.Empty() {
		return FastBuild
	}

	if d.customCommand != "" {
		return CustomBuild
	}

	return UnknownBuild
}

func (s *tiltfileState) dockerBuild(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var dockerRef string
	var contextVal, dockerfilePathVal, buildArgs, dockerfileContentsVal, cacheVal, liveUpdateVal, ignoreVal starlark.Value
	var matchInEnvVars bool
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs,
		"ref", &dockerRef,
		"context", &contextVal,
		"build_args?", &buildArgs,
		"dockerfile?", &dockerfilePathVal,
		"dockerfile_contents?", &dockerfileContentsVal,
		"cache?", &cacheVal,
		"live_update?", &liveUpdateVal,
		"match_in_env_vars?", &matchInEnvVars,
		"ignore?", &ignoreVal,
	); err != nil {
		return nil, err
	}

	ref, err := container.ParseNamed(dockerRef)
	if err != nil {
		return nil, fmt.Errorf("Argument 1 (ref): can't parse %q: %v", dockerRef, err)
	}

	if contextVal == nil {
		return nil, fmt.Errorf("Argument 2 (context): empty but is required")
	}
	context, err := s.localPathFromSkylarkValue(contextVal)
	if err != nil {
		return nil, err
	}

	var sba map[string]string
	if buildArgs != nil {
		d, ok := buildArgs.(*starlark.Dict)
		if !ok {
			return nil, fmt.Errorf("Argument 3 (build_args): expected dict, got %T", buildArgs)
		}

		sba, err = skylarkStringDictToGoMap(d)
		if err != nil {
			return nil, fmt.Errorf("Argument 3 (build_args): %v", err)
		}
	}

	dockerfilePath := context.join("Dockerfile")
	var dockerfileContents string
	if dockerfileContentsVal != nil && dockerfilePathVal != nil {
		return nil, fmt.Errorf("Cannot specify both dockerfile and dockerfile_contents keyword arguments")
	}
	if dockerfileContentsVal != nil {
		switch v := dockerfileContentsVal.(type) {
		case *blob:
			dockerfileContents = v.text
		case starlark.String:
			dockerfileContents = v.GoString()
		default:
			return nil, fmt.Errorf("Argument (dockerfile_contents): must be string or blob.")
		}
	} else if dockerfilePathVal != nil {
		dockerfilePath, err = s.localPathFromSkylarkValue(dockerfilePathVal)
		if err != nil {
			return nil, err
		}

		bs, err := s.readFile(dockerfilePath)
		if err != nil {
			return nil, errors.Wrap(err, "error reading dockerfile")
		}
		dockerfileContents = string(bs)
	} else {
		bs, err := s.readFile(dockerfilePath)
		if err != nil {
			return nil, errors.Wrapf(err, "error reading dockerfile")
		}
		dockerfileContents = string(bs)
	}

	cachePaths, err := s.cachePathsFromSkylarkValue(cacheVal)
	if err != nil {
		return nil, err
	}

	liveUpdate, err := s.liveUpdateFromSteps(liveUpdateVal)
	if err != nil {
		return nil, errors.Wrap(err, "live_update")
	}

	starlarkIgnores := starlarkValueOrSequenceToSlice(ignoreVal)
	var ignores []string
	for _, v := range starlarkIgnores {
		switch val := v.(type) {
		case starlark.String:
			ignores = append(ignores, val.GoString())
		default:
			return nil, fmt.Errorf("ignores must be a string or a sequence of strings; found a %T", val)
		}
	}

	r := &dockerImage{
		dbDockerfilePath: dockerfilePath,
		dbDockerfile:     dockerfile.Dockerfile(dockerfileContents),
		dbBuildPath:      context,
		configurationRef: container.NewRefSelector(ref),
		dbBuildArgs:      sba,
		cachePaths:       cachePaths,
		liveUpdate:       liveUpdate,
		matchInEnvVars:   matchInEnvVars,
		ignores:          ignores,
	}
	err = s.buildIndex.addImage(r)
	if err != nil {
		return nil, err
	}

	// NOTE(maia): docker_build returns a fast build that users can optionally
	// populate; if populated, we use it for in-place updates of this image
	// (but use DockerBuild info for image builds)
	fb := &fastBuild{s: s, img: r}
	return fb, nil
}

func (s *tiltfileState) fastBuildForImage(image *dockerImage) model.FastBuild {
	return model.FastBuild{
		BaseDockerfile: image.baseDockerfile.String(),
		Syncs:          s.syncsToDomain(image),
		Runs:           image.runs,
		Entrypoint:     model.ToShellCmd(image.entrypoint),
		HotReload:      image.hotReload,
	}
}

func (s *tiltfileState) customBuild(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var dockerRef string
	var command string
	var deps *starlark.List
	var tag string
	var disablePush bool
	var liveUpdateVal starlark.Value
	var matchInEnvVars bool

	err := starlark.UnpackArgs(fn.Name(), args, kwargs,
		"ref", &dockerRef,
		"command", &command,
		"deps", &deps,
		"tag?", &tag,
		"disable_push?", &disablePush,
		"live_update?", &liveUpdateVal,
		"match_in_env_vars?", &matchInEnvVars,
	)
	if err != nil {
		return nil, err
	}

	ref, err := reference.ParseNormalizedNamed(dockerRef)
	if err != nil {
		return nil, fmt.Errorf("Argument 1 (ref): can't parse %q: %v", dockerRef, err)
	}

	if command == "" {
		return nil, fmt.Errorf("Argument 2 (command) can't be empty")
	}

	if deps == nil || deps.Len() == 0 {
		return nil, fmt.Errorf("Argument 3 (deps) can't be empty")
	}

	var localDeps []string
	iter := deps.Iterate()
	defer iter.Done()
	var v starlark.Value
	for iter.Next(&v) {
		p, err := s.localPathFromSkylarkValue(v)
		if err != nil {
			return nil, fmt.Errorf("Argument 3 (deps): %v", err)
		}
		localDeps = append(localDeps, p.path)
	}

	liveUpdate, err := s.liveUpdateFromSteps(liveUpdateVal)
	if err != nil {
		return nil, errors.Wrap(err, "live_update")
	}

	img := &dockerImage{
		configurationRef: container.NewRefSelector(ref),
		customCommand:    command,
		customDeps:       localDeps,
		customTag:        tag,
		disablePush:      disablePush,
		liveUpdate:       liveUpdate,
		matchInEnvVars:   matchInEnvVars,
	}

	err = s.buildIndex.addImage(img)
	if err != nil {
		return nil, err
	}

	return &customBuild{s: s, img: img}, nil
}

type customBuild struct {
	s   *tiltfileState
	img *dockerImage
}

var _ starlark.Value = &customBuild{}

func (b *customBuild) String() string {
	return fmt.Sprintf("custom_build(%q)", b.img.configurationRef.String())
}

func (b *customBuild) Type() string {
	return "custom_build"
}

func (b *customBuild) Freeze() {}

func (b *customBuild) Truth() starlark.Bool {
	return true
}

func (b *customBuild) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: custom_build")
}

const (
	addFastBuildN = "add_fast_build"
)

func (b *customBuild) Attr(name string) (starlark.Value, error) {
	switch name {
	case addFastBuildN:
		return starlark.NewBuiltin(name, b.addFastBuild), nil
	default:
		return nil, nil
	}
}

func (b *customBuild) AttrNames() []string {
	return []string{addFastBuildN}
}

func (b *customBuild) addFastBuild(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return &fastBuild{s: b.s, img: b.img}, nil
}

func (s *tiltfileState) fastBuild(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {

	var dockerRef, entrypoint string
	var baseDockerfile starlark.Value
	var cacheVal starlark.Value
	err := starlark.UnpackArgs(fn.Name(), args, kwargs,
		"ref", &dockerRef,
		"base_dockerfile", &baseDockerfile,
		"entrypoint?", &entrypoint,
		"cache?", &cacheVal,
	)
	if err != nil {
		return nil, err
	}

	baseDockerfilePath, err := s.localPathFromSkylarkValue(baseDockerfile)
	if err != nil {
		return nil, fmt.Errorf("Argument 2 (base_dockerfile): %v", err)
	}

	ref, err := container.ParseNamed(dockerRef)
	if err != nil {
		return nil, fmt.Errorf("Parsing %q: %v", dockerRef, err)
	}

	bs, err := s.readFile(baseDockerfilePath)
	if err != nil {
		return nil, errors.Wrap(err, "error reading dockerfile")
	}

	df := dockerfile.Dockerfile(bs)
	if err = df.ValidateBaseDockerfile(); err != nil {
		return nil, err
	}

	cachePaths, err := s.cachePathsFromSkylarkValue(cacheVal)
	if err != nil {
		return nil, err
	}

	r := &dockerImage{
		baseDockerfilePath: baseDockerfilePath,
		baseDockerfile:     df,
		configurationRef:   container.NewRefSelector(ref),
		entrypoint:         entrypoint,
		cachePaths:         cachePaths,
	}
	err = s.buildIndex.addImage(r)
	if err != nil {
		return nil, err
	}

	fb := &fastBuild{s: s, img: r}
	return fb, nil
}

func (s *tiltfileState) cachePathsFromSkylarkValue(val starlark.Value) ([]string, error) {
	if val == nil {
		return nil, nil
	}
	cachePaths := starlarkValueOrSequenceToSlice(val)

	var ret []string
	for _, v := range cachePaths {
		str, ok := v.(starlark.String)
		if !ok {
			return nil, fmt.Errorf("cache param %v is a %T; must be a string", v, v)
		}
		ret = append(ret, string(str))
	}
	return ret, nil
}

type fastBuild struct {
	s   *tiltfileState
	img *dockerImage
}

var _ starlark.Value = &fastBuild{}

func (b *fastBuild) String() string {
	return fmt.Sprintf("fast_build(%q)", b.img.configurationRef.String())
}

func (b *fastBuild) Type() string {
	return "fast_build"
}

func (b *fastBuild) Freeze() {}

func (b *fastBuild) Truth() starlark.Bool {
	return true
}

func (b *fastBuild) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: fast_build")
}

const (
	fbAddN       = "add"
	fbRunN       = "run"
	fbHotReloadN = "hot_reload"
)

func (b *fastBuild) Attr(name string) (starlark.Value, error) {
	switch name {
	case fbAddN:
		return starlark.NewBuiltin(name, b.add), nil
	case fbRunN:
		return starlark.NewBuiltin(name, b.run), nil
	case fbHotReloadN:
		return starlark.NewBuiltin(name, b.hotReload), nil
	default:
		return nil, nil
	}
}

func (b *fastBuild) AttrNames() []string {
	return []string{fbAddN, fbRunN, fbHotReloadN}
}

func (b *fastBuild) hotReload(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}

	b.img.hotReload = true

	return b, nil
}

func (b *fastBuild) add(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(b.img.runs) > 0 {
		return nil, fmt.Errorf("fast_build(%q).add() called after .run(); must add all code before runs", b.img.configurationRef.String())
	}

	var src starlark.Value
	var mountPoint string

	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "src", &src, "dest", &mountPoint); err != nil {
		return nil, err
	}

	s := sync{}
	lp, err := b.s.localPathFromSkylarkValue(src)
	if err != nil {
		return nil, errors.Wrapf(err, "%s.%s(): invalid type for src (arg 1)", b.String(), fn.Name())
	}
	s.src = lp

	s.mountPoint = mountPoint
	b.img.syncs = append(b.img.syncs, s)

	return b, nil
}

func (b *fastBuild) run(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var cmd string
	var trigger starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "cmd", &cmd, "trigger?", &trigger); err != nil {
		return nil, err
	}

	var triggers []string
	switch trigger := trigger.(type) {
	case *starlark.List:
		l := trigger.Len()
		triggers = make([]string, l)
		for i := 0; i < l; i++ {
			t := trigger.Index(i)
			tStr, isStr := t.(starlark.String)
			if !isStr {
				return nil, badTypeErr(fn, starlark.String(""), t)
			}
			triggers[i] = string(tStr)
		}
	case starlark.String:
		triggers = []string{string(trigger)}
	}

	run := model.ToRun(model.ToShellCmd(cmd))
	run = run.WithTriggers(triggers, b.s.absWorkingDir())

	b.img.runs = append(b.img.runs, run)
	return b, nil
}

type sync struct {
	src        localPath
	mountPoint string
}

func (s *tiltfileState) syncsToDomain(image *dockerImage) []model.Sync {
	var result []model.Sync

	for _, sync := range image.syncs {
		result = append(result, model.Sync{LocalPath: sync.src.path, ContainerPath: sync.mountPoint})
	}

	return result
}

func reposForPaths(paths []localPath) []model.LocalGitRepo {
	var result []model.LocalGitRepo
	repoSet := map[string]bool{}

	for _, path := range paths {
		repo := path.repo
		if repo == nil || repoSet[repo.basePath] {
			continue
		}

		repoSet[repo.basePath] = true
		result = append(result, model.LocalGitRepo{
			LocalPath: repo.basePath,
		})
	}

	return result
}

func (s *tiltfileState) reposForImage(image *dockerImage) []model.LocalGitRepo {
	var paths []localPath
	for _, sync := range image.syncs {
		paths = append(paths, sync.src)
	}
	paths = append(paths,
		image.baseDockerfilePath,
		image.dbDockerfilePath,
		image.dbBuildPath,
		s.filename)

	return reposForPaths(paths)
}

func (s *tiltfileState) defaultRegistry(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if s.defaultRegistryHost != "" {
		return starlark.None, errors.New("default registry already defined")
	}

	var dr string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "name", &dr); err != nil {
		return nil, err
	}

	s.defaultRegistryHost = dr

	return starlark.None, nil
}

func (s *tiltfileState) dockerignoresFromPathsAndIgnores(paths []string, ignores []string) []model.Dockerignore {
	var result []model.Dockerignore
	dupeSet := map[string]bool{}

	for _, path := range paths {
		if path == "" || dupeSet[path] {
			continue
		}
		dupeSet[path] = true

		if !ospath.IsDir(path) {
			continue
		}

		result = append(result, model.Dockerignore{
			LocalPath: path,
			Contents:  ignoresToDockerignoreContents(ignores),
		})

		contents, err := s.readFile(s.localPathFromString(filepath.Join(path, ".dockerignore")))
		if err != nil {
			continue
		}

		result = append(result, model.Dockerignore{
			LocalPath: path,
			Contents:  string(contents),
		})
	}

	return result
}

func ignoresToDockerignoreContents(ignores []string) string {
	var output strings.Builder

	for _, ignore := range ignores {
		output.WriteString(ignore)
		output.WriteString("\n")
	}

	return output.String()
}

func (s *tiltfileState) dockerignoresForImage(image *dockerImage) []model.Dockerignore {
	var paths []string

	for _, sync := range image.syncs {
		paths = append(paths, sync.src.path)

		// NOTE(maia): this doesn't reflect the behavior of Docker, which only
		// looks in the build context for the .dockerignore file. Leaving it
		// for now, though, for fastbuild cases where .dockerignore doesn't
		// live in the user's synced dir(s) (e.g. user only syncs several specific
		// files, not a directory containing the .dockerignore).
		repo := sync.src.repo
		if repo != nil {
			paths = append(paths, repo.basePath)
		}
	}
	paths = append(paths, image.dbBuildPath.path)

	return s.dockerignoresFromPathsAndIgnores(paths, image.ignores)
}

func (s *tiltfileState) checkForFastBuilds(manifests []model.Manifest) {
	for _, m := range manifests {
		for _, iTarg := range m.ImageTargets {
			if !iTarg.AnyFastBuildInfo().Empty() {
				s.warnings = append(s.warnings, fastBuildDeprecationWarning)
				return
			}
		}
	}
}

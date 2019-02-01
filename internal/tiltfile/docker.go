package tiltfile

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/docker/distribution/reference"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/dockerfile"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
)

type dockerImage struct {
	baseDockerfilePath localPath
	baseDockerfile     dockerfile.Dockerfile
	ref                reference.Named
	mounts             []mount
	steps              []model.Step
	entrypoint         string
	cachePaths         []string
	hotReload          bool

	staticDockerfilePath localPath
	staticDockerfile     dockerfile.Dockerfile
	staticBuildPath      localPath
	staticBuildArgs      model.DockerBuildArgs

	// Whether this has been matched up yet to a deploy resource.
	matched bool
}

func (s *tiltfileState) dockerBuild(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var dockerRef string
	var contextVal, dockerfilePathVal, buildArgs, cacheVal, dockerfileContentsVal starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs,
		"ref", &dockerRef,
		"context", &contextVal,
		"build_args?", &buildArgs,
		"dockerfile?", &dockerfilePathVal,
		"dockerfile_contents?", &dockerfileContentsVal,
		"cache?", &cacheVal,
	); err != nil {
		return nil, err
	}

	ref, err := reference.ParseNormalizedNamed(dockerRef)
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
			return nil, err
		}
		dockerfileContents = string(bs)
	} else {
		bs, err := s.readFile(dockerfilePath)
		if err != nil {
			return nil, err
		}
		dockerfileContents = string(bs)
	}

	cachePaths, err := s.cachePathsFromSkylarkValue(cacheVal)
	if err != nil {
		return nil, err
	}

	r := &dockerImage{
		staticDockerfilePath: dockerfilePath,
		staticDockerfile:     dockerfile.Dockerfile(dockerfileContents),
		staticBuildPath:      context,
		ref:                  ref,
		staticBuildArgs:      sba,
		cachePaths:           cachePaths,
	}
	err = s.buildIndex.addImage(r)
	if err != nil {
		return nil, err
	}

	return starlark.None, nil
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

	ref, err := reference.ParseNormalizedNamed(dockerRef)
	if err != nil {
		return nil, fmt.Errorf("Parsing %q: %v", dockerRef, err)
	}

	bs, err := s.readFile(baseDockerfilePath)
	if err != nil {
		return nil, err
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
		ref:                ref,
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
	if val, ok := val.(starlark.Sequence); ok {
		var result []string
		it := val.Iterate()
		defer it.Done()
		var i starlark.Value
		for it.Next(&i) {
			str, ok := i.(starlark.String)
			if !ok {
				return nil, fmt.Errorf("cache param %v is a %T; must be a string", i, i)
			}
			result = append(result, string(str))
		}
		return result, nil
	}
	str, ok := val.(starlark.String)
	if !ok {
		return nil, fmt.Errorf("cache param %v is a %T; must be a string or a sequence of strings", val, val)
	}
	return []string{string(str)}, nil
}

type fastBuild struct {
	s   *tiltfileState
	img *dockerImage
}

var _ starlark.Value = &fastBuild{}

func (b *fastBuild) String() string {
	return fmt.Sprintf("fast_build(%q)", b.img.ref.Name())
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
	addN       = "add"
	runN       = "run"
	hotReloadN = "hot_reload"
)

func (b *fastBuild) Attr(name string) (starlark.Value, error) {
	switch name {
	case addN:
		return starlark.NewBuiltin(name, b.add), nil
	case runN:
		return starlark.NewBuiltin(name, b.run), nil
	case hotReloadN:
		return starlark.NewBuiltin(name, b.hotReload), nil
	default:
		return starlark.None, nil
	}
}

func (b *fastBuild) AttrNames() []string {
	return []string{addN, runN}
}

func (b *fastBuild) hotReload(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}

	b.img.hotReload = true

	return b, nil
}

func (b *fastBuild) add(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(b.img.steps) > 0 {
		return nil, fmt.Errorf("fast_build(%q).add() called after .run(); must add all code before runs", b.img.ref.Name())
	}

	var src starlark.Value
	var mountPoint string

	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "src", &src, "dest", &mountPoint); err != nil {
		return nil, err
	}

	m := mount{}
	switch p := src.(type) {
	case localPath:
		m.src = p
	case *gitRepo:
		m.src = p.makeLocalPath("")
	default:
		return nil, fmt.Errorf("fast_build(%q).add(): invalid type for src. Got %s want gitRepo OR localPath", fn.Name(), src.Type())
	}

	m.mountPoint = mountPoint
	b.img.mounts = append(b.img.mounts, m)

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

	step := model.ToStep(b.s.absWorkingDir(), model.ToShellCmd(cmd))
	step.Triggers = triggers

	b.img.steps = append(b.img.steps, step)
	return b, nil
}

type mount struct {
	src        localPath
	mountPoint string
}

func (s *tiltfileState) mountsToDomain(image *dockerImage) []model.Mount {
	var result []model.Mount

	for _, m := range image.mounts {
		result = append(result, model.Mount{LocalPath: m.src.path, ContainerPath: m.mountPoint})
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
			LocalPath:         repo.basePath,
			GitignoreContents: repo.gitignoreContents,
		})
	}

	return result
}

func (s *tiltfileState) reposForImage(image *dockerImage) []model.LocalGitRepo {
	var paths []localPath
	for _, m := range image.mounts {
		paths = append(paths, m.src)
	}
	paths = append(paths,
		image.baseDockerfilePath,
		image.staticDockerfilePath,
		image.staticBuildPath,
		s.filename)

	return reposForPaths(paths)
}

func dockerignoresForPaths(paths []string) []model.Dockerignore {
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

		contents, err := ioutil.ReadFile(filepath.Join(path, ".dockerignore"))
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

func (s *tiltfileState) dockerignoresForImage(image *dockerImage) []model.Dockerignore {
	var paths []string

	for _, m := range image.mounts {
		paths = append(paths, m.src.path)

		repo := m.src.repo
		if repo != nil {
			paths = append(paths, repo.basePath)
		}
	}
	paths = append(paths, image.staticBuildPath.path)

	return dockerignoresForPaths(paths)
}

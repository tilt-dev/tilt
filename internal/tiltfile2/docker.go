package tiltfile2

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/docker/distribution/reference"
	"github.com/google/skylark"

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
	tiltfilePath       localPath
	cachePaths         []string

	staticDockerfilePath localPath
	staticDockerfile     dockerfile.Dockerfile
	staticBuildPath      localPath
	staticBuildArgs      model.DockerBuildArgs
}

func (s *tiltfileState) dockerBuild(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var dockerRef string
	var contextVal, dockerfilePathVal, buildArgs, cacheVal skylark.Value
	if err := skylark.UnpackArgs(fn.Name(), args, kwargs,
		"ref", &dockerRef,
		"context", &contextVal,
		"build_args?", &buildArgs,
		"dockerfile?", &dockerfilePathVal,
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

	dockerfilePath := context.join("Dockerfile")
	if dockerfilePathVal != nil {
		dockerfilePath, err = s.localPathFromSkylarkValue(dockerfilePathVal)
		if err != nil {
			return nil, err
		}
	}

	var sba map[string]string
	if buildArgs != nil {
		d, ok := buildArgs.(*skylark.Dict)
		if !ok {
			return nil, fmt.Errorf("Argument 3 (build_args): expected dict, got %T", buildArgs)
		}

		sba, err = skylarkStringDictToGoMap(d)
		if err != nil {
			return nil, fmt.Errorf("Argument 3 (build_args): %v", err)
		}
	}

	bs, err := s.readFile(dockerfilePath)
	if err != nil {
		return nil, err
	}

	if s.imagesByName[ref.Name()] != nil {
		return nil, fmt.Errorf("Image for ref %q has already been defined", ref.Name())
	}

	cachePaths, err := s.cachePathsFromSkylarkValue(cacheVal)
	if err != nil {
		return nil, err
	}

	r := &dockerImage{
		staticDockerfilePath: dockerfilePath,
		staticDockerfile:     dockerfile.Dockerfile(bs),
		staticBuildPath:      context,
		ref:                  ref,
		tiltfilePath:         s.filename,
		staticBuildArgs:      sba,
		cachePaths:           cachePaths,
	}
	s.imagesByName[ref.Name()] = r
	s.images = append(s.images, r)

	return skylark.None, nil
}

func (s *tiltfileState) fastBuild(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {

	var dockerRef, entrypoint string
	var baseDockerfile skylark.Value
	var cacheVal skylark.Value
	err := skylark.UnpackArgs(fn.Name(), args, kwargs,
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

	if s.imagesByName[ref.Name()] != nil {
		return nil, fmt.Errorf("Image for ref %q has already been defined", ref.Name())
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
		tiltfilePath:       s.filename,
	}
	s.imagesByName[ref.Name()] = r
	s.images = append(s.images, r)

	fb := &fastBuild{s: s, img: r}
	return fb, nil
}

func (s *tiltfileState) cachePathsFromSkylarkValue(val skylark.Value) ([]string, error) {
	if val == nil {
		return nil, nil
	}
	if val, ok := val.(skylark.Sequence); ok {
		var result []string
		it := val.Iterate()
		defer it.Done()
		var i skylark.Value
		for it.Next(&i) {
			str, ok := i.(skylark.String)
			if !ok {
				return nil, fmt.Errorf("cache param %v is a %T; must be a string", i, i)
			}
			result = append(result, string(str))
		}
		return result, nil
	}
	str, ok := val.(skylark.String)
	if !ok {
		return nil, fmt.Errorf("cache param %v is a %T; must be a string or a sequence of strings", val, val)
	}
	return []string{string(str)}, nil
}

type fastBuild struct {
	s   *tiltfileState
	img *dockerImage
}

var _ skylark.Value = &fastBuild{}

func (b *fastBuild) String() string {
	return fmt.Sprintf("fast_build(%q)", b.img.ref.Name())
}

func (b *fastBuild) Type() string {
	return "fast_build"
}

func (b *fastBuild) Freeze() {}

func (b *fastBuild) Truth() skylark.Bool {
	return true
}

func (b *fastBuild) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: fast_build")
}

const (
	addN = "add"
	runN = "run"
)

func (b *fastBuild) Attr(name string) (skylark.Value, error) {
	switch name {
	case addN:
		return skylark.NewBuiltin(name, b.add), nil
	case runN:
		return skylark.NewBuiltin(name, b.run), nil
	default:
		return skylark.None, nil
	}
}

func (b *fastBuild) AttrNames() []string {
	return []string{addN, runN}
}

func (b *fastBuild) add(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	if len(b.img.steps) > 0 {
		return nil, fmt.Errorf("fast_build(%q).add() called after .run(); must add all code before runs", b.img.ref.Name())
	}

	var src skylark.Value
	var mountPoint string

	if err := skylark.UnpackArgs(fn.Name(), args, kwargs, "src", &src, "dest", &mountPoint); err != nil {
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

func (b *fastBuild) run(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var cmd string
	var trigger skylark.Value
	if err := skylark.UnpackArgs(fn.Name(), args, kwargs, "cmd", &cmd, "trigger?", &trigger); err != nil {
		return nil, err
	}

	var triggers []string
	switch trigger := trigger.(type) {
	case *skylark.List:
		l := trigger.Len()
		triggers = make([]string, l)
		for i := 0; i < l; i++ {
			t := trigger.Index(i)
			tStr, isStr := t.(skylark.String)
			if !isStr {
				return nil, badTypeErr(fn, skylark.String(""), t)
			}
			triggers[i] = string(tStr)
		}
	case skylark.String:
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

func (s *tiltfileState) reposToDomain(image *dockerImage) []model.LocalGithubRepo {
	var result []model.LocalGithubRepo
	repoSet := map[string]bool{}

	maybeAddRepo := func(path localPath) {
		repo := path.repo
		if repo == nil || repoSet[repo.basePath] {
			return
		}

		repoSet[repo.basePath] = true
		result = append(result, model.LocalGithubRepo{
			LocalPath:         repo.basePath,
			GitignoreContents: repo.gitignoreContents,
		})
	}

	for _, m := range image.mounts {
		maybeAddRepo(m.src)
	}
	maybeAddRepo(image.baseDockerfilePath)
	maybeAddRepo(image.staticDockerfilePath)
	maybeAddRepo(image.staticBuildPath)
	maybeAddRepo(image.tiltfilePath)

	return result
}

func (s *tiltfileState) dockerignoresToDomain(image *dockerImage) []model.Dockerignore {
	var result []model.Dockerignore
	dupeSet := map[string]bool{}

	maybeAddDockerignore := func(path string) {
		if path == "" || dupeSet[path] {
			return
		}
		dupeSet[path] = true

		if !ospath.IsDir(path) {
			return
		}

		contents, err := ioutil.ReadFile(filepath.Join(path, ".dockerignore"))
		if err != nil {
			return
		}

		result = append(result, model.Dockerignore{
			LocalPath: path,
			Contents:  string(contents),
		})
	}

	for _, m := range image.mounts {
		maybeAddDockerignore(m.src.path)

		repo := m.src.repo
		if repo != nil {
			maybeAddDockerignore(repo.basePath)
		}
	}
	maybeAddDockerignore(image.staticBuildPath.path)

	return result
}

package tiltfile2

import (
	"context"
)

type tiltfileState struct {
	// set at creation
	ctx context.Context

	// added to during execution
	configFiles []string
	images      map[string]*dockerImage
	k8s         map[string]*k8sResource

	// current fast_build_state
	buildCtx *dockerImage

	// final execution error, if any
	err error
}

func newTiltfileState(ctx context.Context) *tiltfileState {
	return &tiltfileState{
		ctx:    context,
		images: make(map[string]*dockerImage),
		k8s:    make(map[string]*k8sResource),
	}
}

func (s *tiltfileState) exec(filename string) {
	filename, err := ospath.RealAbs(filename)
	if err != nil {
		s.err = err
		return
	}

	_, err = skylark.ExecFile(thread, filename, nil, s.builtins())
	s.err = err
}

// Builtin functions

const (
	dockerBuildN = "docker_build"
	k8sResourceN = "k8s_resource"
)

func (s *TiltfileState) builtins() skylark.StringDict {
	r := make(skylark.StringDict)
	add := func(name string, fn func(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error)) {
		r[name] = skylark.NewBuiltin(name, fn)
	}

	r.add(dockerBuildN, s.dockerBuild)
	r.add(k8sResourceN, s.k8sResource)
	return r
}

func (s *TiltfileState) dockerBuild(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var dockerRef string
	var dockerfilePath, buildPath, buildArgs skylark.Value
	if err := skylark.UnpackArgs(fn.Name(), args, kwargs,
		"ref", &dockerRef,
		"dockerfile", &dockerfilePath,
		"build_args?", &buildArgs,
		"context?", &buildPath,
	); err != nil {
		return nil, err
	}

	ref, err := reference.ParseNormalizedNamed(dockerRef)
	if err != nil {
		return nil, fmt.Errorf("Parsing %q: %v", dockerRef, err)
	}

	dockerfileLocalPath, err := s.localPathFromSkylarkValue(dockerfilePath)
	if err != nil {
		return nil, fmt.Errorf("Argument 1 (dockerfile): %v", err)
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

	var buildLocalPath localPath
	if buildPath == nil {
		buildLocalPath = localPath{
			path: filepath.Dir(dockerfileLocalPath.path),
			repo: dockerfileLocalPath.repo,
		}
	} else {
		buildLocalPath, err = s.localPathFromSkylarkValue(buildPath)
		if err != nil {
			return nil, fmt.Errorf("Argument 4 (context): %v", err)
		}
	}

	s.recordConfigFile(dockerfileLocalPath.path)
	bs, err := ioutil.ReadFile(dockerfileLocalPath.path)
	if err != nil {
		return nil, err
	}

	s.images[ref.Name()] = &dockerImage{
		staticDockerfilePath: dockerfileLocalPath,
		staticDockerfile:     dockerfile.Dockerfile(bs),
		staticBuildPath:      buildLocalPath,
		ref:                  ref,
		tiltFilename:         s.filename,
		staticBuildArgs:      sba,
	}

	return nil, nil
}

func (s *TiltfileState) k8sResource(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var name string
	var yaml skylark.Value

	s.k8s[name] = &k8sResource{}
}

func (s *TiltfileState) analyze() {
	// first, get the right kubernetes yaml in the right k8sResources
}

func (s *TiltfileState) translate() ([]model.Manifest, error) {
}

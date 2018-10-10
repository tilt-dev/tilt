package tiltfile

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/pkg/errors"

	"github.com/docker/distribution/reference"
	"github.com/google/skylark"
	"github.com/google/skylark/resolve"
	"github.com/windmilleng/tilt/internal/model"
)

const FileName = "Tiltfile"

type Tiltfile struct {
	globals  skylark.StringDict
	filename string
	thread   *skylark.Thread
}

func init() {
	resolve.AllowLambda = true
	resolve.AllowNestedDef = true
}

func (t *Tiltfile) makeSkylarkDockerImage(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var dockerfileName, entrypoint, dockerRef string
	err := skylark.UnpackArgs(fn.Name(), args, kwargs,
		"docker_file_name", &dockerfileName,
		"docker_file_tag", &dockerRef,
		"entrypoint?", &entrypoint,
	)
	if err != nil {
		return nil, err
	}

	ref, err := reference.ParseNormalizedNamed(dockerRef)
	if err != nil {
		return nil, fmt.Errorf("Parsing %q: %v", dockerRef, err)
	}

	existingBC := thread.Local(buildContextKey)

	if existingBC != nil {
		return skylark.None, errors.New("tried to start a build context while another build context was already open")
	}

	buildContext := &dockerImage{
		baseDockerfilePath: dockerfileName,
		ref:                ref,
		entrypoint:         entrypoint,
		tiltFilename:       t.filename,
	}
	err = recordReadFile(thread, dockerfileName)
	if err != nil {
		return skylark.None, err
	}
	thread.SetLocal(buildContextKey, buildContext)
	return skylark.None, nil
}

func (t *Tiltfile) makeStaticBuild(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var dockerfilePath, dockerRef, buildPath string
	err := skylark.UnpackArgs(fn.Name(), args, kwargs,
		"dockerfile", &dockerfilePath,
		"ref", &dockerRef,
		"context?", &buildPath,
	)
	if err != nil {
		return nil, err
	}

	ref, err := reference.ParseNormalizedNamed(dockerRef)
	if err != nil {
		return nil, fmt.Errorf("Parsing %q: %v", dockerRef, err)
	}

	if buildPath == "" {
		buildPath = filepath.Dir(dockerfilePath)
	}

	buildPath, err = filepath.Abs(buildPath)
	if err != nil {
		return skylark.None, err
	}

	buildContext := &dockerImage{
		staticDockerfilePath: dockerfilePath,
		staticBuildPath:      buildPath,
		ref:                  ref,
		tiltFilename:         t.filename,
	}
	err = recordReadFile(thread, dockerfilePath)
	if err != nil {
		return skylark.None, err
	}
	return buildContext, nil
}

func unimplementedSkylarkFunction(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	return skylark.None, errors.New(fmt.Sprintf("%s not implemented", fn.Name()))
}

func makeSkylarkK8Manifest(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var yaml skylark.String
	var dockerImage *dockerImage
	err := skylark.UnpackArgs(fn.Name(), args, kwargs, "yaml", &yaml, "dockerImage", &dockerImage)
	if err != nil {
		return nil, err
	}
	// Name will be initialized later
	return &k8sManifest{
		k8sYaml:     yaml,
		dockerImage: *dockerImage,
	}, nil
}

func makeSkylarkCompositeManifest(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {

	var manifestFuncs skylark.Iterable
	err := skylark.UnpackArgs(fn.Name(), args, kwargs,
		"services", &manifestFuncs)
	if err != nil {
		return nil, err
	}

	var manifests []*k8sManifest

	var v skylark.Value
	i := manifestFuncs.Iterate()
	defer i.Done()
	for i.Next(&v) {
		switch v := v.(type) {
		case *skylark.Function:
			thread.SetLocal(readFilesKey, []string{})
			r, err := v.Call(thread, nil, nil)
			if err != nil {
				return nil, err
			}
			s, ok := r.(*k8sManifest)
			if !ok {
				return nil, fmt.Errorf("composite_service: function %v returned %v %T; expected k8s_service", v.Name(), r, r)
			}
			err = recordReadToTiltFile(thread)
			if err != nil {
				return nil, err
			}

			files, err := getAndClearReadFiles(thread)
			if err != nil {
				return nil, err
			}

			s.name = v.Name()
			s.configFiles = files

			manifests = append(manifests, s)
		default:
			return nil, fmt.Errorf("composite_service: unexpected input %v %T", v, v)
		}
	}
	return compManifest{manifests}, nil
}

func makeSkylarkGitRepo(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var path string
	err := skylark.UnpackArgs(fn.Name(), args, kwargs, "path", &path)
	if err != nil {
		return nil, err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("filepath.Abs: %v", err)
	}

	_, err = os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("Reading path %s: %v", path, err)
	}

	if _, err := os.Stat(filepath.Join(absPath, ".git")); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s isn't a valid git repo: it doesn't have a .git/ directory", absPath)
	}

	gitignoreContents, err := ioutil.ReadFile(filepath.Join(absPath, ".gitignore"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	dockerignoreContents, err := ioutil.ReadFile(filepath.Join(absPath, ".dockerignore"))
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	repo := gitRepo{absPath, string(gitignoreContents), string(dockerignoreContents)}

	return repo, nil
}

func runLocalCmd(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var command string
	err := skylark.UnpackArgs(fn.Name(), args, kwargs, "command", &command)
	if err != nil {
		return nil, err
	}

	out, err := exec.Command("sh", "-c", command).Output()
	if err != nil {
		errorMessage := fmt.Sprintf("command '%v' failed.\nerror: '%v'\nstdout: '%v'", command, err, string(out))
		exitError, ok := err.(*exec.ExitError)
		if ok {
			errorMessage += fmt.Sprintf("\nstderr: '%v'", string(exitError.Stderr))
		}
		return nil, errors.New(errorMessage)
	}
	return skylark.String(out), nil
}

func readFile(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var path string
	err := skylark.UnpackArgs(fn.Name(), args, kwargs, "path", &path)
	if err != nil {
		return nil, err
	}

	dat, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	err = recordReadFile(thread, path)
	if err != nil {
		return nil, err
	}

	return skylark.String(dat), nil
}

func stopBuild(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	buildContext, err := getAndClearBuildContext(thread)
	if err != nil {
		return nil, err
	} else if buildContext == nil {
		return nil, errors.New(noActiveBuildError)
	}
	return buildContext, nil
}

func Load(filename string, out io.Writer) (*Tiltfile, error) {
	thread := &skylark.Thread{
		Print: func(_ *skylark.Thread, msg string) {
			_, _ = fmt.Fprintln(out, msg)
		},
	}

	tiltfile := &Tiltfile{
		filename: filename,
		thread:   thread,
	}

	predeclared := skylark.StringDict{
		"start_fast_build":  skylark.NewBuiltin("start_fast_build", tiltfile.makeSkylarkDockerImage),
		"start_slow_build":  skylark.NewBuiltin("start_slow_build", unimplementedSkylarkFunction),
		"static_build":      skylark.NewBuiltin("static_build", tiltfile.makeStaticBuild),
		"k8s_service":       skylark.NewBuiltin("k8s_service", makeSkylarkK8Manifest),
		"local_git_repo":    skylark.NewBuiltin("local_git_repo", makeSkylarkGitRepo),
		"local":             skylark.NewBuiltin("local", runLocalCmd),
		"composite_service": skylark.NewBuiltin("composite_service", makeSkylarkCompositeManifest),
		"read_file":         skylark.NewBuiltin("read_file", readFile),
		"stop_build":        skylark.NewBuiltin("stop_build", stopBuild),
		"add":               skylark.NewBuiltin("add", addMount),
		"run":               skylark.NewBuiltin("run", runDockerImageCmd),
	}

	globals, err := skylark.ExecFile(thread, filename, nil, predeclared)
	if err != nil {
		return nil, err
	}

	tiltfile.globals = globals
	return tiltfile, nil
}

func (tiltfile Tiltfile) GetManifestConfigs(manifestName string) ([]model.Manifest, error) {
	f, ok := tiltfile.globals[manifestName]

	if !ok {
		return nil, fmt.Errorf("%v does not define a global named '%v'", tiltfile.filename, manifestName)
	}

	manifestFunction, ok := f.(*skylark.Function)

	if !ok {
		return nil, fmt.Errorf("'%v' is a '%v', not a function. service definitions must be functions", manifestName, f.Type())
	}

	if manifestFunction.NumParams() != 0 {
		return nil, fmt.Errorf("func '%v' is defined to take more than 0 arguments. service definitions must take 0 arguments", manifestName)
	}

	thread := tiltfile.thread
	thread.SetLocal(readFilesKey, []string{})

	err := recordReadToTiltFile(thread)
	if err != nil {
		return nil, err
	}

	val, err := manifestFunction.Call(tiltfile.thread, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("error running '%v': %v", manifestName, err.Error())
	}

	files, err := getAndClearReadFiles(thread)
	if err != nil {
		return nil, err
	}

	switch manifest := val.(type) {
	case compManifest:
		var servs []model.Manifest

		for _, cServ := range manifest.cManifest {
			s, err := skylarkManifestToDomain(cServ)
			if err != nil {
				return nil, err
			}

			servs = append(servs, s)
		}
		return servs, nil
	case *k8sManifest:
		manifest.configFiles = files

		s, err := skylarkManifestToDomain(manifest)
		if err != nil {
			return nil, err
		}

		s.Name = model.ManifestName(manifestName)
		return []model.Manifest{s}, nil

	default:
		return nil, fmt.Errorf("'%v' returned a '%v', but it needs to return a k8s_service or composite_service", manifestName, val.Type())
	}
}

func skylarkManifestToDomain(manifest *k8sManifest) (model.Manifest, error) {
	k8sYaml, ok := skylark.AsString(manifest.k8sYaml)
	if !ok {
		return model.Manifest{}, fmt.Errorf("internal error: k8sService.k8sYaml was not a string in '%v'", manifest)
	}

	var err error
	image := manifest.dockerImage
	baseDockerfileBytes := []byte{}
	staticDockerfileBytes := []byte{}
	if image.staticDockerfilePath != "" {
		staticDockerfileBytes, err = ioutil.ReadFile(image.staticDockerfilePath)
		if err != nil {
			return model.Manifest{}, fmt.Errorf("failed to open dockerfile '%v': %v", image.staticDockerfilePath, err)
		}
	} else {
		baseDockerfileBytes, err = ioutil.ReadFile(image.baseDockerfilePath)
		if err != nil {
			return model.Manifest{}, fmt.Errorf("failed to open dockerfile '%v': %v", image.baseDockerfilePath, err)
		}
	}

	return model.Manifest{
		K8sYaml:        k8sYaml,
		BaseDockerfile: string(baseDockerfileBytes),
		Mounts:         skylarkMountsToDomain(image.mounts),
		Steps:          image.steps,
		Entrypoint:     model.ToShellCmd(image.entrypoint),
		DockerRef:      image.ref,
		Name:           model.ManifestName(manifest.name),
		TiltFilename:   image.tiltFilename,
		ConfigFiles:    SkylarkConfigFilesToDomain(manifest.configFiles),

		StaticDockerfile: string(staticDockerfileBytes),
		StaticBuildPath:  string(image.staticBuildPath),

		Repos:        SkylarkReposToDomain(image.mounts),
		PortForwards: manifest.portForwards,
	}, nil

}

func SkylarkConfigFilesToDomain(cf []string) []string {
	ss := sort.StringSlice(cf)
	ss.Sort()

	return ss
}

func SkylarkReposToDomain(sMount []mount) []model.LocalGithubRepo {
	dRepos := []model.LocalGithubRepo{}
	for _, m := range sMount {
		if m.repo.Truth() {
			dRepos = append(dRepos, model.LocalGithubRepo{
				LocalPath:            m.repo.basePath,
				DockerignoreContents: m.repo.dockerignoreContents,
				GitignoreContents:    m.repo.gitignoreContents,
			})
		}
	}

	return dRepos
}

func skylarkMountsToDomain(sMounts []mount) []model.Mount {
	dMounts := make([]model.Mount, len(sMounts))
	for i, m := range sMounts {
		dMounts[i] = model.Mount{
			LocalPath:     m.src.path,
			ContainerPath: m.mountPoint,
		}
	}
	return dMounts
}

func recordReadToTiltFile(t *skylark.Thread) error {
	err := recordReadFile(t, FileName)
	if err != nil {
		return err
	}

	return nil
}

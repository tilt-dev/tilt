package tiltfile

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/google/skylark"
	"github.com/google/skylark/resolve"
	"github.com/windmilleng/tilt/internal/dockerignore"
	"github.com/windmilleng/tilt/internal/git"
	"github.com/windmilleng/tilt/internal/model"
)

const FileName = "Tiltfile"
const buildContextKey = "buildContext"

type Tiltfile struct {
	globals  skylark.StringDict
	filename string
	thread   *skylark.Thread
}

func init() {
	resolve.AllowLambda = true
	resolve.AllowNestedDef = true
}

func makeSkylarkDockerImage(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var dockerfileName, entrypoint, dockerfileTag string
	err := skylark.UnpackArgs(fn.Name(), args, kwargs,
		"docker_file_name", &dockerfileName,
		"docker_file_tag", &dockerfileTag,
		"entrypoint?", &entrypoint,
	)
	if err != nil {
		return nil, err
	}

	tag, err := reference.ParseNormalizedNamed(dockerfileTag)
	if err != nil {
		return nil, fmt.Errorf("Parsing %q: %v", dockerfileTag, err)
	}

	existingBC := thread.Local(buildContextKey)

	if existingBC != nil {
		return skylark.None, errors.New("tried to start a build context while another build context was already open")
	}

	buildContext := &dockerImage{dockerfileName, tag, []mount{}, []model.Step{}, entrypoint, []model.PathMatcher{git.FalseIgnoreTester{}}}
	thread.SetLocal(buildContextKey, buildContext)
	return skylark.None, nil
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
	return k8sManifest{yaml, *dockerImage, ""}, nil
}

func makeSkylarkCompositeManifest(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {

	var manifestFuncs skylark.Iterable
	err := skylark.UnpackArgs(fn.Name(), args, kwargs,
		"services", &manifestFuncs)
	if err != nil {
		return nil, err
	}

	var manifests []k8sManifest

	var v skylark.Value
	i := manifestFuncs.Iterate()
	defer i.Done()
	for i.Next(&v) {
		switch v := v.(type) {
		case *skylark.Function:
			r, err := v.Call(thread, nil, nil)
			if err != nil {
				return nil, err
			}
			s, ok := r.(k8sManifest)
			if !ok {
				return nil, fmt.Errorf("composite_service: function %v returned %v %T; expected k8s_service", v.Name(), r, r)
			}
			s.name = v.Name()
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

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	t1, err := git.NewRepoIgnoreTester(ctx, absPath)
	if err != nil {
		return nil, err
	}
	t2, err := dockerignore.NewDockerIgnoreTester(absPath)
	if err != nil {
		return nil, err
	}

	ct := model.NewCompositeMatcher([]model.PathMatcher{t1, t2})

	return gitRepo{absPath, ct}, nil
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

	return skylark.String(dat), nil
}

func stopBuild(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	buildContext, ok := thread.Local(buildContextKey).(*dockerImage)
	if !ok {
		return nil, errors.New("internal error: buildContext thread local was not of type *dockerImage")
	}
	thread.SetLocal(buildContextKey, nil)

	return buildContext, nil
}

func Load(filename string, out io.Writer) (*Tiltfile, error) {
	thread := &skylark.Thread{
		Print: func(_ *skylark.Thread, msg string) {
			_, _ = fmt.Fprintln(out, msg)
		},
	}

	predeclared := skylark.StringDict{
		"start_fast_build":  skylark.NewBuiltin("start_fast_build", makeSkylarkDockerImage),
		"start_slow_build":  skylark.NewBuiltin("start_slow_build", unimplementedSkylarkFunction),
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

	return &Tiltfile{globals, filename, thread}, nil
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

	val, err := manifestFunction.Call(tiltfile.thread, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("error running '%v': %v", manifestName, err.Error())
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
	case k8sManifest:
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

func skylarkManifestToDomain(manifest k8sManifest) (model.Manifest, error) {
	k8sYaml, ok := skylark.AsString(manifest.k8sYaml)
	if !ok {
		return model.Manifest{}, fmt.Errorf("internal error: k8sService.k8sYaml was not a string in '%v'", manifest)
	}

	dockerFileBytes, err := ioutil.ReadFile(manifest.dockerImage.fileName)
	if err != nil {
		return model.Manifest{}, fmt.Errorf("failed to open dockerfile '%v': %v", manifest.dockerImage.fileName, err)
	}

	return model.Manifest{
		K8sYaml:        k8sYaml,
		DockerfileText: string(dockerFileBytes),
		Mounts:         skylarkMountsToDomain(manifest.dockerImage.mounts),
		Steps:          manifest.dockerImage.steps,
		Entrypoint:     model.ToShellCmd(manifest.dockerImage.entrypoint),
		DockerfileTag:  manifest.dockerImage.fileTag,
		Name:           model.ManifestName(manifest.name),
		FileFilter:     model.NewCompositeMatcher(manifest.dockerImage.filters),
	}, nil

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

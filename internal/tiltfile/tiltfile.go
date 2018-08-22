package tiltfile

import (
	"errors"
	"fmt"
	"os/exec"

	"io/ioutil"

	"github.com/google/skylark"
	"github.com/windmilleng/tilt/internal/model"
)

type Tiltfile struct {
	globals  skylark.StringDict
	filename string
	thread   *skylark.Thread
}

func makeSkylarkDockerImage(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	// TODO(maia): `entrypoint` should be an optional arg; user should be able to declare
	// it in base Dockerfile and Tilt should be able to parse it out and apply it.
	var dockerfileName, entrypoint, dockerfileTag string
	err := skylark.UnpackArgs(fn.Name(), args, kwargs,
		"docker_file_name", &dockerfileName,
		"docker_file_tag", &dockerfileTag,
		"entrypoint", &entrypoint,
	)
	if err != nil {
		return nil, err
	}
	return &dockerImage{dockerfileName, dockerfileTag, []mount{}, []string{}, entrypoint}, nil
}

func makeSkylarkK8Service(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var yaml skylark.String
	var dockerImage *dockerImage
	err := skylark.UnpackArgs(fn.Name(), args, kwargs, "yaml", &yaml, "dockerImage", &dockerImage)
	if err != nil {
		return nil, err
	}
	// Name will be initialized later
	return k8sService{yaml, *dockerImage, ""}, nil
}

func makeSkylarkCompositeService(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	// TODO - validate args / err handling / or change the format of the input
	m := args[0].(*skylark.Dict)
	var k8sServArray []k8sService

	for _, name := range m.Keys() {
		service, _, err := m.Get(name)
		if err != nil {
			return nil, err
		}
		k8sServ, ok := service.(k8sService)
		if !ok {
			return nil, fmt.Errorf("error: arguments in composite_service are not of type k8s_service '%+v'", args)
		}
		k8sServ.name, ok = skylark.AsString(name)
		if !ok {
			return nil, fmt.Errorf("'%v' is a '%v', not a string. service definitions must be strings", name, name.Type())
		}
		k8sServArray = append(k8sServArray, k8sServ)
	}
	return compService{k8sServArray}, nil
}

func makeSkylarkGitRepo(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var path string
	err := skylark.UnpackArgs(fn.Name(), args, kwargs, "path", &path)
	if err != nil {
		return nil, err
	}

	return gitRepo{path}, nil
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

func Load(filename string) (*Tiltfile, error) {
	thread := &skylark.Thread{
		Print: func(_ *skylark.Thread, msg string) { fmt.Println(msg) },
	}

	predeclared := skylark.StringDict{
		"build_docker_image": skylark.NewBuiltin("build_docker_image", makeSkylarkDockerImage),
		"k8s_service":        skylark.NewBuiltin("k8s_service", makeSkylarkK8Service),
		"local_git_repo":     skylark.NewBuiltin("local_git_repo", makeSkylarkGitRepo),
		"local":              skylark.NewBuiltin("local", runLocalCmd),
		"composite_service":  skylark.NewBuiltin("composite_service", makeSkylarkCompositeService),
	}

	globals, err := skylark.ExecFile(thread, filename, nil, predeclared)
	if err != nil {
		return nil, err
	}

	return &Tiltfile{globals, filename, thread}, nil
}

func (tiltfile Tiltfile) GetServiceConfigs(serviceName string) ([]model.Service, error) {
	f, ok := tiltfile.globals[serviceName]

	if !ok {
		return nil, fmt.Errorf("%v does not define a global named '%v'", tiltfile.filename, serviceName)
	}

	serviceFunction, ok := f.(*skylark.Function)

	if !ok {
		return nil, fmt.Errorf("'%v' is a '%v', not a function. service definitions must be functions", serviceName, f.Type())
	}

	if serviceFunction.NumParams() != 0 {
		return nil, fmt.Errorf("func '%v' is defined to take more than 0 arguments. service definitions must take 0 arguments", serviceName)
	}

	val, err := serviceFunction.Call(tiltfile.thread, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("error running '%v': %v", serviceName, err.Error())
	}

	switch service := val.(type) {
	case compService:
		var servs []model.Service

		for _, cServ := range service.cService {
			s, err := skylarkServiceToDomain(cServ)
			if err != nil {
				return nil, err
			}

			servs = append(servs, s)
		}
		return servs, nil
	case k8sService:
		s, err := skylarkServiceToDomain(service)
		if err != nil {
			return nil, err
		}
		s.Name = model.ServiceName(serviceName)
		return []model.Service{s}, nil

	default:
		return nil, fmt.Errorf("'%v' returned a '%v', but it needs to return a k8s_service or composite_service", serviceName, val.Type())
	}
}

func skylarkServiceToDomain(service k8sService) (model.Service, error) {
	k8sYaml, ok := skylark.AsString(service.k8sYaml)
	if !ok {
		return model.Service{}, fmt.Errorf("internal error: k8sService.k8sYaml was not a string in '%v'", service)
	}

	dockerFileBytes, err := ioutil.ReadFile(service.dockerImage.fileName)
	if err != nil {
		return model.Service{}, fmt.Errorf("failed to open dockerfile '%v': %v", service.dockerImage.fileName, err)
	}

	return model.Service{
		K8sYaml:        k8sYaml,
		DockerfileText: string(dockerFileBytes),
		Mounts:         skylarkMountsToDomain(service.dockerImage.mounts),
		Steps:          model.ToShellCmds(service.dockerImage.cmds),
		Entrypoint:     model.ToShellCmd(service.dockerImage.entrypoint),
		DockerfileTag:  service.dockerImage.fileTag,
		Name:           model.ServiceName(service.name),
	}, nil

}

func skylarkMountsToDomain(sMounts []mount) []model.Mount {
	dMounts := make([]model.Mount, len(sMounts))
	for i, m := range sMounts {
		dMounts[i] = model.Mount{
			Repo:          model.LocalGithubRepo{LocalPath: m.repo.path},
			ContainerPath: m.mountPoint,
		}
	}
	return dMounts
}

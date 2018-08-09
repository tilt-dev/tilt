package tiltfile

import (
	"errors"
	"fmt"
	"github.com/google/skylark"
	"github.com/windmilleng/tilt/internal/proto"
	"io/ioutil"
	"os/exec"
)

type Tiltfile struct {
	globals  skylark.StringDict
	filename string
	thread   *skylark.Thread
}

func makeSkylarkDockerImage(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var dockerfileName, dockerfileTag skylark.String
	err := skylark.UnpackArgs(fn.Name(), args, kwargs, "docker_file_name", &dockerfileName, "docker_file_tag", &dockerfileTag)
	if err != nil {
		return nil, err
	}
	return &dockerImage{dockerfileName, dockerfileTag, []mount{}, []string{}}, nil
}

func makeSkylarkK8Service(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var yaml skylark.String
	var dockerImage *dockerImage
	err := skylark.UnpackArgs(fn.Name(), args, kwargs, "yaml", &yaml, "dockerImage", &dockerImage)
	if err != nil {
		return nil, err
	}

	return k8sService{yaml, *dockerImage}, nil
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

	out, err := exec.Command("bash", "-c", command).Output()
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
		"git_repo":			  skylark.NewBuiltin("git_repo", makeSkylarkGitRepo),
		"local":              skylark.NewBuiltin("local", runLocalCmd),
	}

	globals, err := skylark.ExecFile(thread, filename, nil, predeclared)
	if err != nil {
		return nil, err
	}

	return &Tiltfile{globals, filename, thread}, nil
}

func (tiltfile Tiltfile) GetServiceConfig(serviceName string) (*proto.Service, error) {
	f, ok := tiltfile.globals[serviceName]

	if !ok {
		return nil, fmt.Errorf("%v does not define a global named '%v'", tiltfile.filename, serviceName)
	}

	serviceFunction, ok := f.(*skylark.Function)

	if !ok {
		return nil, fmt.Errorf("'%v' is a '%v', not a function. service definitions must be functions.", serviceName, f.Type())
	}

	if serviceFunction.NumParams() != 0 {
		return nil, fmt.Errorf("'%v' is defined to take more than 0 arguments. service definitions must take 0 arguments.", serviceName)
	}

	val, err := serviceFunction.Call(tiltfile.thread, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("error running '%v': %v", serviceName, err.Error())
	}

	service, ok := val.(k8sService)
	if !ok {
		return nil, fmt.Errorf("'%v' returned a '%v', but it needs to return a k8s_service.", serviceName, val.Type())
	}

	k8sYaml, ok := skylark.AsString(service.k8sYaml)
	if !ok {
		return nil, fmt.Errorf("internal error: k8sService.k8sYaml was not a string in '%v'", service)
	}

	dockerFileName, ok := skylark.AsString(service.dockerImage.fileName)
	if !ok {
		return nil, fmt.Errorf("internal error: k8sService.dockerFileName was not a string in '%v'", service)
	}

	dockerFileBytes, err := ioutil.ReadFile(dockerFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to open dockerfile '%v': %v", dockerFileName, err)
	}

	dockerFileText := string(dockerFileBytes)

	dockerFileTag, ok := skylark.AsString(service.dockerImage.fileTag)
	if !ok {
		return nil, fmt.Errorf("internal error: k8sService.dockerFileTag was not a string in '%v'", service)
	}

	mounts := make([]*proto.Mount, 0, len(service.dockerImage.mounts))
	for _, mount := range service.dockerImage.mounts {
		repo := proto.Repo{&proto.Repo_GitRepo{&proto.GitRepo{mount.repo.path}}}
		mounts = append(mounts, &proto.Mount{Repo: &repo, ContainerPath: mount.mount_point})
	}

	dockerCmds := make([]*proto.Cmd, 0, len(service.dockerImage.cmds))
	for _, cmd := range service.dockerImage.cmds {
		dockerCmds = append(dockerCmds, &proto.Cmd{Argv: []string{"bash", "-c", cmd}})
	}

	return &proto.Service{K8SYaml: k8sYaml, DockerfileText: dockerFileText, Mounts: mounts, Steps: dockerCmds, DockerfileTag: dockerFileTag}, nil
}

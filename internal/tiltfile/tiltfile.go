package tiltfile

import (
	"fmt"
	"github.com/google/skylark"
	"github.com/windmilleng/tilt/internal/proto"
	"io/ioutil"
)

type Tiltfile struct {
	globals  skylark.StringDict
	filename string
	thread   *skylark.Thread
}

func makeSkylarkDockerImage(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var dockerfileName, dockerfileTag skylark.String
	err := skylark.UnpackPositionalArgs("build_docker_image", args, kwargs, 2, &dockerfileName, &dockerfileTag)
	if err != nil {
		return nil, err
	}
	return &dockerImage{dockerfileName, dockerfileTag, skylark.List{}}, nil
}

func makeSkylarkK8Service(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var yaml skylark.String
	var dockerImage *dockerImage
	err := skylark.UnpackPositionalArgs("k8s_service", args, kwargs, 2, &yaml, &dockerImage)
	if err != nil {
		return nil, err
	}

	return k8sService{yaml, *dockerImage}, nil
}

func Load(filename string) (*Tiltfile, error) {
	thread := &skylark.Thread{
		Print: func(_ *skylark.Thread, msg string) { fmt.Println(msg) },
	}

	predeclared := skylark.StringDict{
		"build_docker_image": skylark.NewBuiltin("build_docker_image", makeSkylarkDockerImage),
		"k8s_service": skylark.NewBuiltin("k8s_service", makeSkylarkK8Service),
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

	dockerCmds := make([]*proto.Cmd, 0, service.dockerImage.cmds.Len())
	iter := service.dockerImage.cmds.Iterate()
	defer iter.Done()
	var cmdValue skylark.Value
	for iter.Next(&cmdValue) {
		cmd, _ := skylark.AsString(cmdValue)
		if !ok {
			return nil, fmt.Errorf("internal error: cmd '%v' was not a string in '%v'", cmdValue, service)
		}
		dockerCmds = append(dockerCmds, &proto.Cmd{Argv: []string{"bash", "-c", cmd}})
	}

	return &proto.Service{K8SYaml: k8sYaml, DockerfileText: dockerFileText, Mounts: []*proto.Mount{}, Steps: dockerCmds, DockerfileTag: dockerFileTag}, nil
}

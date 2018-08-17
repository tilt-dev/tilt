package tiltfile

import (
	"errors"
	"fmt"
	"os/exec"

	"io/ioutil"

	"github.com/google/skylark"
	"github.com/windmilleng/tilt/internal/proto"
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
		"git_repo":           skylark.NewBuiltin("git_repo", makeSkylarkGitRepo),
		"local":              skylark.NewBuiltin("local", runLocalCmd),
	}

	globals, err := skylark.ExecFile(thread, filename, nil, predeclared)
	if err != nil {
		return nil, err
	}

	return &Tiltfile{globals, filename, thread}, nil
}

func (tiltfile Tiltfile) GetServiceConfig(serviceName string) ([]*proto.Service, error) { // array
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

<<<<<<< HEAD
=======
	compService, ok := val.(compService)
	if ok {
		for _, cServ := range compService.cService {

			k8sYaml, ok := skylark.AsString(cServ.k8sYaml)
			if !ok {
				return nil, fmt.Errorf("internal error: k8sService.k8sYaml was not a string in '%v'", cServ)
			}

			dockerFileBytes, err := ioutil.ReadFile(cServ.dockerImage.fileName)
			if err != nil {
				return nil, fmt.Errorf("failed to open dockerfile '%v': %v", cServ.dockerImage.fileName, err)
			}

			mounts := make([]*proto.Mount, 0, len(cServ.dockerImage.mounts))
			for _, mount := range cServ.dockerImage.mounts {
				repo := proto.Repo{RepoType: &proto.Repo_GitRepo{GitRepo: &proto.GitRepo{LocalPath: mount.repo.path}}}
				mounts = append(mounts, &proto.Mount{Repo: &repo, ContainerPath: mount.mountPoint})
			}

			dockerCmds := make([]*proto.Cmd, 0, len(cServ.dockerImage.cmds))
			for _, cmd := range cServ.dockerImage.cmds {
				dockerCmds = append(dockerCmds, toShellCmd(cmd))
			}

			var protoServ []proto.Service

			protoServ = append(protoServ, proto.Service{
				K8SYaml:        k8sYaml,
				DockerfileText: string(dockerFileBytes),
				Mounts:         mounts,
				Steps:          dockerCmds,
				Entrypoint:     toShellCmd(cServ.dockerImage.entrypoint),
				DockerfileTag:  cServ.dockerImage.fileTag,
			})

			return protoServ, nil
		}
	}
	if !ok {
		_, ok := val.(k8sService)
		if !ok {
			return nil, fmt.Errorf("'%v' returned a '%v', but it needs to return a k8s_service", serviceName, val.Type())
		}
		return nil, fmt.Errorf("'%v' returned a '%v', but it needs to return a k8s_service or composite_service", serviceName, val.Type())
	}

>>>>>>> 4b5755d... Slowly turning Service into []Service
	service, ok := val.(k8sService)
	if !ok {
		return nil, fmt.Errorf("'%v' returned a '%v', but it needs to return a k8s_service", serviceName, val.Type())
	}

	k8sYaml, ok := skylark.AsString(service.k8sYaml)
	if !ok {
		return nil, fmt.Errorf("internal error: k8sService.k8sYaml was not a string in '%v'", service)
	}

	dockerFileBytes, err := ioutil.ReadFile(service.dockerImage.fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to open dockerfile '%v': %v", service.dockerImage.fileName, err)
	}

	mounts := make([]*proto.Mount, 0, len(service.dockerImage.mounts))
	for _, mount := range service.dockerImage.mounts {
		repo := proto.Repo{RepoType: &proto.Repo_GitRepo{GitRepo: &proto.GitRepo{LocalPath: mount.repo.path}}}
		mounts = append(mounts, &proto.Mount{Repo: &repo, ContainerPath: mount.mountPoint})
	}

	dockerCmds := make([]*proto.Cmd, 0, len(service.dockerImage.cmds))
	for _, cmd := range service.dockerImage.cmds {
		dockerCmds = append(dockerCmds, toShellCmd(cmd))
	}

	var protoServ []proto.Service

	protoServ = append(protoServ, proto.Service{
		K8SYaml:        k8sYaml,
		DockerfileText: string(dockerFileBytes),
		Mounts:         mounts,
		Steps:          dockerCmds,
		Entrypoint:     toShellCmd(service.dockerImage.entrypoint),
		DockerfileTag:  service.dockerImage.fileTag,
	})

	return protoServ, nil
}

func toShellCmd(cmd string) *proto.Cmd {
	return &proto.Cmd{Argv: []string{"sh", "-c", cmd}}
}

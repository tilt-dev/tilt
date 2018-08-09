package tiltfile

import (
	"github.com/google/skylark"
	"fmt"
	"errors"
)

type k8sService struct {
	k8sYaml skylark.String
	dockerImage dockerImage
}

func (service k8sService) String() string {
	return fmt.Sprintf("[k8sService] (yaml omitted) dockerImage: %v", service.dockerImage)
}

func (service k8sService) Type() string {
	return "k8sService"
}

func (service k8sService) Freeze() {
	service.k8sYaml.Freeze()
	service.dockerImage.Freeze()
}

func (service k8sService) Truth() skylark.Bool {
	return true
}

func (service k8sService) Hash() (uint32, error) {
	h1, err := service.k8sYaml.Hash()
	if err != nil {
		return 0, err
	}
	h2, err := service.dockerImage.Hash()
	if err != nil {
		return 0, err
	}

	return h1 * 17 + h2 * 39, nil
}

type dockerImage struct {
	fileName skylark.String
	fileTag skylark.String
	cmds skylark.List
}

func add_docker_image_cmd(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var cmd skylark.String
	skylark.UnpackPositionalArgs(fn.Name(), args, kwargs, 1, &cmd)
	image, ok := fn.Receiver().(*dockerImage)
	if !ok {
		return nil, errors.New("internal error: add_docker_image_cmd called on non-dockerImage")
	}
	image.cmds.Append(cmd)
	return skylark.None, nil
}

func (dockerImage *dockerImage) String() string {
	return fmt.Sprintf("fileName: %v, fileTag: %v, cmds: %v", dockerImage.fileName, dockerImage.fileTag, dockerImage.cmds)
}

func (dockerImage *dockerImage) Type() string {
	return "dockerImage"
}

func (dockerImage *dockerImage) Freeze() {
	dockerImage.fileName.Freeze()
	dockerImage.fileTag.Freeze()
	dockerImage.cmds.Freeze()
}

func (dockerImage *dockerImage) Truth() skylark.Bool {
	return true
}

func (dockerImage *dockerImage) Hash() (uint32, error) {
	h1, err := dockerImage.fileName.Hash()
	if err != nil {
		return 0, err
	}
	h2, err := dockerImage.fileTag.Hash()
	if err != nil {
		return 0, err
	}
	h3, err := dockerImage.cmds.Hash()
	if err != nil {
		return 0, err
	}
	return h1 * 17 + h2 * 39 + h3 * 19, nil
}

func (dockerImage *dockerImage) Attr(name string) (skylark.Value, error) {
	switch name {
	case "file_name":
		return dockerImage.fileName, nil
	case "file_tag":
		return dockerImage.fileTag, nil
	case "add_cmd":
		return skylark.NewBuiltin("add_cmd", add_docker_image_cmd).BindReceiver(dockerImage), nil
	default:
		return nil, nil
	}
}

func (dockerImage *dockerImage) AttrNames() []string {
	return []string{"file_name", "file_tag", "add_cmd"}
}


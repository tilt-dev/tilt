package tiltfile

import (
	"errors"
	"fmt"
	"github.com/google/skylark"
)

type k8sService struct {
	k8sYaml     skylark.String
	dockerImage dockerImage
}

var _ skylark.Value = k8sService{}

func (s k8sService) String() string {
	shortYaml := s.k8sYaml.String()
	const maxYamlCharsToInclude = 40
	if len(shortYaml) > maxYamlCharsToInclude {
		shortYaml = shortYaml[:maxYamlCharsToInclude]
	}
	return fmt.Sprintf("[k8sService] yaml: '%v' dockerImage: '%v'", shortYaml, s.dockerImage)
}

func (s k8sService) Type() string {
	return "k8sService"
}

func (s k8sService) Freeze() {
	s.k8sYaml.Freeze()
	s.dockerImage.Freeze()
}

func (k8sService) Truth() skylark.Bool {
	return true
}

func (k8sService) Hash() (uint32, error) {
	return 0, errors.New("unhashable type: k8sService")
}

type mount struct {
	mount_point string
	repo gitRepo
}

type dockerImage struct {
	fileName skylark.String
	fileTag  skylark.String
	mounts	 []mount
	cmds     []string
}

var _ skylark.Value = &dockerImage{}

func addDockerImageCmd(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var skylarkCmd skylark.String
	err := skylark.UnpackArgs(fn.Name(), args, kwargs, "cmd", &skylarkCmd)
	if err != nil {
		return nil, err
	}
	image, ok := fn.Receiver().(*dockerImage)
	if !ok {
		return nil, errors.New("internal error: add_docker_image_cmd called on non-dockerImage")
	}

	cmd, ok := skylark.AsString(skylarkCmd)
	if !ok {
		return nil, errors.New("internal error: skylarkCmd was not a string")
	}
	image.cmds = append(image.cmds, cmd)
	return skylark.None, nil
}

func addMount(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var mountPoint string
	var gitRepo gitRepo
	err := skylark.UnpackArgs(fn.Name(), args, kwargs, "mount_point", &mountPoint, "git_repo", &gitRepo)
	if err != nil {
		return nil, err
	}

	image, ok := fn.Receiver().(*dockerImage)
	if !ok {
		return nil, errors.New("internal error: add_docker_image_cmd called on non-dockerImage")
	}

	image.mounts = append(image.mounts, mount{mountPoint, gitRepo})

	return skylark.None, nil
}

func (d *dockerImage) String() string {
	return fmt.Sprintf("fileName: %v, fileTag: %v, cmds: %v", d.fileName, d.fileTag, d.cmds)
}

func (d *dockerImage) Type() string {
	return "dockerImage"
}

func (d *dockerImage) Freeze() {
}

func (*dockerImage) Truth() skylark.Bool {
	return true
}

func (*dockerImage) Hash() (uint32, error) {
	return 0, errors.New("unhashable type: dockerImage")
}

func (d *dockerImage) Attr(name string) (skylark.Value, error) {
	switch name {
	case "file_name":
		return d.fileName, nil
	case "file_tag":
		return d.fileTag, nil
	case "add_cmd":
		return skylark.NewBuiltin(name, addDockerImageCmd).BindReceiver(d), nil
	case "add_mount":
		return skylark.NewBuiltin(name, addMount).BindReceiver(d), nil
	default:
		return nil, nil
	}
}

func (*dockerImage) AttrNames() []string {
	return []string{"file_name", "file_tag", "add_cmd"}
}

type gitRepo struct {
	path string
}

var _ skylark.Value = gitRepo{}

func (gr gitRepo) String() string {
	return fmt.Sprintf("[gitRepo] '%v'", gr.path)
}

func (gr gitRepo) Type() string {
	return "gitRepo"
}

func (gr gitRepo) Freeze() {
}

func (gitRepo) Truth() skylark.Bool {
	return true
}

func (gitRepo) Hash() (uint32, error) {
	return 0, errors.New("unhashable type: gitRepo")
}
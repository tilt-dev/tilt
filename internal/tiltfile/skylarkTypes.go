package tiltfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/google/skylark"
	"github.com/windmilleng/tilt/internal/dockerignore"
	"github.com/windmilleng/tilt/internal/model"
)

const oldMountSyntaxError = "The syntax for `add` has changed. Before it was `.add(dest, src)`. Now it is `.add(src, dest)`."

type compManifest struct {
	cManifest []k8sManifest
}

var _ skylark.Value = compManifest{}

func (s compManifest) String() string {
	return fmt.Sprintf("composite manifest: %+v", s.cManifest)
}
func (s compManifest) Type() string {
	return "compManifest"
}
func (s compManifest) Freeze() {
}
func (compManifest) Truth() skylark.Bool {
	return true
}
func (compManifest) Hash() (uint32, error) {
	return 0, errors.New("unhashable type: composite manifest")
}

type k8sManifest struct {
	k8sYaml     skylark.String
	dockerImage dockerImage
	name        string
}

var _ skylark.Value = k8sManifest{}

func (s k8sManifest) String() string {
	shortYaml := s.k8sYaml.String()
	const maxYamlCharsToInclude = 40
	if len(shortYaml) > maxYamlCharsToInclude {
		shortYaml = shortYaml[:maxYamlCharsToInclude]
	}
	return fmt.Sprintf("[k8sManifest] yaml: '%v' dockerImage: '%v'", shortYaml, s.dockerImage)
}

func (s k8sManifest) Type() string {
	return "k8sManifest"
}

func (s k8sManifest) Freeze() {
	s.k8sYaml.Freeze()
	s.dockerImage.Freeze()
}

func (k8sManifest) Truth() skylark.Bool {
	return true
}

func (k8sManifest) Hash() (uint32, error) {
	return 0, errors.New("unhashable type: k8sManifest")
}

type mount struct {
	mountPoint string
	repo       gitRepo
}

type dockerImage struct {
	fileName   string
	fileTag    reference.Named
	mounts     []mount
	steps      []model.Step
	entrypoint string
}

var _ skylark.Value = &dockerImage{}

func runDockerImageCmd(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var skylarkCmd skylark.String
	var trigger skylark.Value
	err := skylark.UnpackArgs(fn.Name(), args, kwargs, "cmd", &skylarkCmd, "trigger?", &trigger)
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

	// TODO(dmiller): in practice, this is the directory that the Tiltfile exists in. It will error otherwise.
	// We should formalize this.
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	step := model.ToStep(model.ToShellCmd(cmd))

	if len(triggers) > 0 {
		pm, err := dockerignore.NewDockerPatternMatcher(cwd, triggers)
		if err != nil {
			return nil, err
		}
		step.Trigger = pm
	}

	image.steps = append(image.steps, step)
	return skylark.None, nil
}

func addMount(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var gitRepo gitRepo
	var mountPoint string
	if len(fn.Receiver().(*dockerImage).steps) > 0 {
		return nil, errors.New("add mount before run command")
	}
	err := skylark.UnpackArgs(fn.Name(), args, kwargs, "src", &gitRepo, "dest", &mountPoint)
	if err != nil {
		if strings.Contains(err.Error(), "add: for parameter 1: got string, want gitRepo") {
			return nil, fmt.Errorf(oldMountSyntaxError)
		}
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
	return fmt.Sprintf("fileName: %v, fileTag: %v, cmds: %v", d.fileName, d.fileTag, d.steps)
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
		return skylark.String(d.fileName), nil
	case "file_tag":
		return skylark.String(d.fileTag.String()), nil
	case "run":
		return skylark.NewBuiltin(name, runDockerImageCmd).BindReceiver(d), nil
	case "add":
		return skylark.NewBuiltin(name, addMount).BindReceiver(d), nil
	default:
		return nil, nil
	}
}

func (*dockerImage) AttrNames() []string {
	return []string{"file_name", "file_tag", "run"}
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

func badTypeErr(b *skylark.Builtin, ex interface{}, v skylark.Value) error {
	return fmt.Errorf("%v expects a %T; got %T (%v)", b.Name(), ex, v, v)
}

func (gr gitRepo) Attr(name string) (skylark.Value, error) {
	switch name {
	case "path":
		return skylark.NewBuiltin(name, fullRepoPath).BindReceiver(gr), nil
	default:
		return nil, nil
	}

}

func (gr gitRepo) AttrNames() []string {
	return []string{"path"}
}

func fullRepoPath(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	r, ok := fn.Receiver().(gitRepo)
	if !ok {
		return nil, errors.New("internal error: path called on non-gitRepo")
	}

	var path skylark.String
	err := skylark.UnpackArgs(fn.Name(), args, kwargs, "path", &path)
	if err != nil {
		return nil, err
	}

	return skylark.String(filepath.Join(r.path, string(path))), nil
}

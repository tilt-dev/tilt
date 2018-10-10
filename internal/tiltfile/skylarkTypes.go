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

const oldMountSyntaxError = "The syntax for `add` has changed. Before it was `add(dest: string, src: string)`. Now it is `add(src: localPath, dest: string)`."
const noActiveBuildError = "No active build"

type compManifest struct {
	cManifest []*k8sManifest
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
	k8sYaml      skylark.String
	dockerImage  dockerImage
	name         string
	configFiles  []string
	portForwards []model.PortForward
}

var _ skylark.Value = &k8sManifest{}
var _ skylark.HasAttrs = &k8sManifest{}

func (k *k8sManifest) String() string {
	shortYaml := k.k8sYaml.String()
	const maxYamlCharsToInclude = 40
	if len(shortYaml) > maxYamlCharsToInclude {
		shortYaml = shortYaml[:maxYamlCharsToInclude]
	}
	return fmt.Sprintf("[k8sManifest] yaml: '%v' dockerImage: '%v'", shortYaml, k.dockerImage)
}

func (k *k8sManifest) Type() string {
	return "k8sManifest"
}

func (k *k8sManifest) Freeze() {
	k.k8sYaml.Freeze()
	k.dockerImage.Freeze()
}

func (k *k8sManifest) Truth() skylark.Bool {
	return true
}

func (k *k8sManifest) Hash() (uint32, error) {
	return 0, errors.New("unhashable type: k8sManifest")
}

func (k *k8sManifest) Attr(name string) (skylark.Value, error) {
	switch name {
	case "port_forward":
		return skylark.NewBuiltin(name, k.createPortForward), nil
	default:
		return nil, nil
	}
}

func (k *k8sManifest) AttrNames() []string {
	return []string{"port_forward"}
}

func (k *k8sManifest) createPortForward(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var localPort int
	var containerPort int
	err := skylark.UnpackArgs(fn.Name(), args, kwargs, "local", &localPort, "remote?", &containerPort)
	if err != nil {
		return nil, err
	}

	k.portForwards = append(k.portForwards, model.PortForward{
		LocalPort:     localPort,
		ContainerPort: containerPort,
	})
	return skylark.None, nil
}

type mount struct {
	src        localPath
	mountPoint string
	repo       gitRepo
}

// See model.Manifest for more information on what all these fields mean.
type dockerImage struct {
	baseDockerfilePath string
	ref                reference.Named
	mounts             []mount
	steps              []model.Step
	entrypoint         string
	tiltFilename       string

	staticDockerfilePath string
	staticBuildPath      string
}

var _ skylark.Value = &dockerImage{}
var _ skylark.HasAttrs = &dockerImage{}

func runDockerImageCmd(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var skylarkCmd skylark.String
	var trigger skylark.Value
	err := skylark.UnpackArgs(fn.Name(), args, kwargs, "cmd", &skylarkCmd, "trigger?", &trigger)
	if err != nil {
		return nil, err
	}
	buildContext, ok := thread.Local("buildContext").(*dockerImage)
	if buildContext == nil {
		return nil, errors.New("run called without a build context")
	}
	if !ok {
		return nil, errors.New("internal error: buildContext thread local was not of type *dockerImage")
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

	buildContext.steps = append(buildContext.steps, step)
	return skylark.None, nil
}

func addMount(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var src skylark.Value
	var mountPoint string

	buildContext, ok := thread.Local("buildContext").(*dockerImage)
	if buildContext == nil {
		return nil, errors.New("add called without a build context")
	}
	if !ok {
		return nil, errors.New("internal error: buildContext thread local was not of type *dockerImage")
	}

	if len(buildContext.steps) > 0 {
		return nil, errors.New("add mount before run command")
	}
	err := skylark.UnpackArgs(fn.Name(), args, kwargs, "src", &src, "dest", &mountPoint)
	if err != nil {
		if strings.Contains(err.Error(), "got gitRepo, want string") {
			return nil, fmt.Errorf(oldMountSyntaxError)
		}
		return nil, err
	}

	m := mount{}
	switch p := src.(type) {
	case localPath:
		m.src = p
	case gitRepo:
		m.src = localPath{p.basePath, p}
		m.repo = p
	default:
		return nil, fmt.Errorf("invalid type for src. Got %s want gitRepo OR localPath", src.Type())
	}
	m.mountPoint = mountPoint
	buildContext.mounts = append(buildContext.mounts, m)

	return skylark.None, nil
}

func (d *dockerImage) String() string {
	if d.baseDockerfilePath != "" {
		return fmt.Sprintf("fileName: %v, ref: %s, cmds: %v", d.baseDockerfilePath, d.ref, d.steps)
	} else {
		return fmt.Sprintf("fileName: %s, path: %s", d.staticDockerfilePath, d.staticBuildPath)
	}
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
		if d.staticDockerfilePath != "" {
			return skylark.String(d.staticDockerfilePath), nil
		} else {
			return skylark.String(d.baseDockerfilePath), nil
		}
	case "file_tag":
		return skylark.String(d.ref.String()), nil
	default:
		return nil, nil
	}
}

func (*dockerImage) AttrNames() []string {
	return []string{"file_name", "file_tag"}
}

type gitRepo struct {
	basePath             string
	gitignoreContents    string
	dockerignoreContents string
}

var _ skylark.Value = gitRepo{}

func (gr gitRepo) String() string {
	return fmt.Sprintf("[gitRepo] '%v'", gr.basePath)
}

func (gr gitRepo) Type() string {
	return "gitRepo"
}

func (gr gitRepo) Freeze() {}

func (gr gitRepo) Truth() skylark.Bool {
	return gr.basePath != "" || gr.gitignoreContents != "" || gr.dockerignoreContents != ""
}

func (gitRepo) Hash() (uint32, error) {
	return 0, errors.New("unhashable type: gitRepo")
}

func (gr gitRepo) Attr(name string) (skylark.Value, error) {
	switch name {
	case "path":
		return skylark.NewBuiltin(name, gr.path), nil
	default:
		return nil, nil
	}

}

func (gr gitRepo) AttrNames() []string {
	return []string{"path"}
}

func (gr gitRepo) path(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var path string
	err := skylark.UnpackArgs(fn.Name(), args, kwargs, "path", &path)
	if err != nil {
		return nil, err
	}

	return localPath{filepath.Join(gr.basePath, path), gr}, nil
}

type localPath struct {
	path string
	repo gitRepo
}

var _ skylark.Value = localPath{}

func (l localPath) String() string {
	return l.path
}

func (localPath) Type() string {
	return "localPath"
}

func (localPath) Freeze() {}

func (localPath) Hash() (uint32, error) {
	return 0, errors.New("unhashable type: localPath")
}

func (localPath) Truth() skylark.Bool {
	return true
}

func badTypeErr(b *skylark.Builtin, ex interface{}, v skylark.Value) error {
	return fmt.Errorf("%v expects a %T; got %T (%v)", b.Name(), ex, v, v)
}

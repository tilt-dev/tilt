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
	src        localPath
	mountPoint string
}

type dockerImage struct {
	fileName   string
	fileTag    reference.Named
	mounts     []mount
	steps      []model.Step
	entrypoint string
	filters    []model.PathMatcher
}

var _ skylark.Value = &dockerImage{}

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

	var lp localPath
	switch p := src.(type) {
	case localPath:
		lp = p
	case gitRepo:
		lp = localPath{p.basePath, p}
	default:
		return nil, fmt.Errorf("invalid type for src. Got %s want gitRepo OR localPath", src.Type())
	}

	buildContext.mounts = append(buildContext.mounts, mount{lp, mountPoint})
	buildContext.filters = append(buildContext.filters, lp.repo.pathMatcher)

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
	default:
		return nil, nil
	}
}

func (*dockerImage) AttrNames() []string {
	return []string{"file_name", "file_tag", "run"}
}

type gitRepo struct {
	basePath    string
	pathMatcher model.PathMatcher
}

var _ skylark.Value = gitRepo{}

func (gr gitRepo) String() string {
	return fmt.Sprintf("[gitRepo] '%v'", gr.basePath)
}

func (gr gitRepo) Type() string {
	return "gitRepo"
}

func (gr gitRepo) Freeze() {}

func (gitRepo) Truth() skylark.Bool {
	return true
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

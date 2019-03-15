package tiltfile

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/windmilleng/tilt/internal/sliceutils"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/kustomize"
	"github.com/windmilleng/tilt/internal/ospath"
)

type gitRepo struct {
	basePath          string
	gitignoreContents string
}

func (s *tiltfileState) newGitRepo(path string) (*gitRepo, error) {
	absPath := s.absPath(path)
	_, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("Reading path %s: %v", path, err)
	}

	if _, err := os.Stat(filepath.Join(absPath, ".git")); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s isn't a valid git repo: it doesn't have a .git/ directory", absPath)
	}

	gitignoreContents, err := ioutil.ReadFile(filepath.Join(absPath, ".gitignore"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return &gitRepo{basePath: absPath, gitignoreContents: string(gitignoreContents)}, nil
}

func (s *tiltfileState) localGitRepo(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path)
	if err != nil {
		return nil, err
	}

	return s.newGitRepo(path)
}

var _ starlark.Value = &gitRepo{}

func (gr *gitRepo) String() string {
	return fmt.Sprintf("[gitRepo] '%v'", gr.basePath)
}

func (gr *gitRepo) Type() string {
	return "gitRepo"
}

func (gr *gitRepo) Freeze() {}

func (gr *gitRepo) Truth() starlark.Bool {
	return gr.basePath != "" || gr.gitignoreContents != ""
}

func (*gitRepo) Hash() (uint32, error) {
	return 0, errors.New("unhashable type: gitRepo")
}

func (gr *gitRepo) Attr(name string) (starlark.Value, error) {
	switch name {
	case "path":
		return starlark.NewBuiltin(name, gr.path), nil
	default:
		return nil, nil
	}

}

func (gr *gitRepo) AttrNames() []string {
	return []string{"path"}
}

func (gr *gitRepo) path(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path)
	if err != nil {
		return nil, err
	}

	return gr.makeLocalPath(path), nil
}

func (gr *gitRepo) makeLocalPath(path string) localPath {
	return localPath{filepath.Join(gr.basePath, path), gr}
}

type localPath struct {
	path string
	repo *gitRepo
}

// If this is the root of a git repo, automatically attach a gitRepo
// so that we don't end up tracking .git changes
func (s *tiltfileState) maybeAttachGitRepo(lp localPath, repoRoot string) localPath {
	if ospath.IsDir(filepath.Join(repoRoot, ".git")) {
		repo, err := s.newGitRepo(repoRoot)
		if err == nil {
			lp.repo = repo
		}
	}
	return lp
}

func (s *tiltfileState) localPathFromString(p string) localPath {
	lp := localPath{path: s.absPath(p)}
	lp = s.maybeAttachGitRepo(lp, lp.path)
	return lp
}

func (s *tiltfileState) localPathFromSkylarkValue(v starlark.Value) (localPath, error) {
	switch v := v.(type) {
	case localPath:
		return v, nil
	case *gitRepo:
		return v.makeLocalPath("."), nil
	case starlark.String:
		return s.localPathFromString(string(v)), nil
	default:
		return localPath{}, fmt.Errorf("expected localPath | gitRepo | string. Actual type: %T", v)
	}
}

var _ starlark.Value = localPath{}

func (lp localPath) String() string {
	return lp.path
}

func (localPath) Type() string {
	return "localPath"
}

func (localPath) Freeze() {}

func (localPath) Hash() (uint32, error) {
	return 0, errors.New("unhashable type: localPath")
}

func (lp localPath) Empty() bool {
	return lp.path == ""
}

func (lp localPath) Truth() starlark.Bool {
	return lp != localPath{}
}

func (lp localPath) join(path string) localPath {
	return localPath{path: filepath.Join(lp.path, path), repo: lp.repo}
}

// When running the Tilt demo, the current working directory is arbitrary.
// So we want to resolve paths relative to the dir where the Tiltfile lives,
// not relative to the working directory.
func (s *tiltfileState) absPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(s.absWorkingDir(), path)
}

func (s *tiltfileState) absWorkingDir() string {
	return filepath.Dir(s.filename.path)
}

func (s *tiltfileState) recordConfigFile(f string) {
	s.configFiles = sliceutils.AppendWithoutDupes(s.configFiles, f)
}

func (s *tiltfileState) readFile(p localPath) ([]byte, error) {
	s.recordConfigFile(p.path)
	return ioutil.ReadFile(p.path)
}

func (s *tiltfileState) skylarkReadFile(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.Value
	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path)
	if err != nil {
		return nil, err
	}

	p, err := s.localPathFromSkylarkValue(path)
	if err != nil {
		return nil, fmt.Errorf("invalid type for path: %v", err)
	}

	bs, err := s.readFile(p)
	if err != nil {
		return nil, err
	}

	return newBlob(string(bs), fmt.Sprintf("file: %s", p.path)), nil
}

type blob struct {
	text   string
	source string
}

var _ starlark.Value = &blob{}

func newBlob(text string, source string) *blob {
	return &blob{text: text, source: source}
}

func (b *blob) String() string {
	return b.text
}

func (b *blob) Type() string {
	return "blob"
}

func (b *blob) Freeze() {}

func (b *blob) Truth() starlark.Bool {
	return len(b.text) > 0
}

func (b *blob) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: blob")
}

func (s *tiltfileState) local(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var command string
	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "command", &command)
	if err != nil {
		return nil, err
	}

	s.logger.Infof("Running `%q`", command)
	out, err := s.execLocalCmd(command)
	if err != nil {
		return nil, err
	}

	return newBlob(out, fmt.Sprintf("cmd: '%s'", command)), nil
}

func (s *tiltfileState) execLocalCmd(cmd string) (string, error) {
	// TODO(nick): Should this also inject any docker.Env overrides?
	c := exec.Command("sh", "-c", cmd)
	c.Dir = filepath.Dir(s.filename.path)
	out, err := c.Output()
	if err != nil {
		errorMessage := fmt.Sprintf("command '%v' failed.\nerror: '%v'\nstdout: '%v'", cmd, err, string(out))
		exitError, ok := err.(*exec.ExitError)
		if ok {
			errorMessage += fmt.Sprintf("\nstderr: '%v'", string(exitError.Stderr))
		}
		return "", errors.New(errorMessage)
	}

	return string(out), nil
}

func (s *tiltfileState) execLocalCmdArgv(argv ...string) (string, error) {
	c := exec.Command(argv[0], argv[1:]...)
	c.Dir = filepath.Dir(s.filename.path)
	out, err := c.Output()
	if err != nil {
		errorMessage := fmt.Sprintf("command '%v' failed.\nerror: '%v'\nstdout: '%v'", argv, err, string(out))
		exitError, ok := err.(*exec.ExitError)
		if ok {
			errorMessage += fmt.Sprintf("\nstderr: '%v'", string(exitError.Stderr))
		}
		return "", errors.New(errorMessage)
	}

	return string(out), nil
}

func (s *tiltfileState) kustomize(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.Value
	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path)
	if err != nil {
		return nil, err
	}

	kustomizePath, err := s.localPathFromSkylarkValue(path)
	if err != nil {
		return nil, fmt.Errorf("Argument 0 (path): %v", err)
	}

	cmd := fmt.Sprintf("kustomize build %s", path)
	yaml, err := s.execLocalCmd(cmd)
	if err != nil {
		return nil, err
	}
	deps, err := kustomize.Deps(kustomizePath.String())
	if err != nil {
		return nil, fmt.Errorf("internal error: %v", err)
	}
	for _, d := range deps {
		s.recordConfigFile(d)
	}

	return newBlob(yaml, fmt.Sprintf("kustomize: %s", kustomizePath.String())), nil
}

func (s *tiltfileState) helm(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.Value
	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path)
	if err != nil {
		return nil, err
	}

	localPath, err := s.localPathFromSkylarkValue(path)
	if err != nil {
		return nil, fmt.Errorf("Argument 0 (path): %v", err)
	}

	yaml, err := s.execLocalCmdArgv("helm", "template", localPath.path)
	if err != nil {
		return nil, err
	}

	s.recordConfigFile(localPath.path)

	return newBlob(string(yaml), fmt.Sprintf("helm: %s", localPath.path)), nil
}

func (s *tiltfileState) blob(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var input starlark.String
	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "input", &input)
	if err != nil {
		return nil, err
	}

	return newBlob(input.GoString(), "Tiltfile blob() call"), nil
}

func (s *tiltfileState) listdir(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var dir starlark.String
	var recursive bool
	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "dir", &dir, "recursive?", &recursive)
	if err != nil {
		return nil, err
	}

	var files []string
	err = filepath.Walk(dir.GoString(), func(path string, info os.FileInfo, err error) error {
		if path == dir.GoString() {
			return nil
		}
		if !info.IsDir() {
			files = append(files, path)
		} else if info.IsDir() && !recursive {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var ret []starlark.Value
	for _, f := range files {
		ret = append(ret, starlark.String(f))
	}

	return starlark.NewList(ret), nil
}

func (s *tiltfileState) decodeJSON(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var jsonString starlark.String
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "json", &jsonString); err != nil {
		return nil, err
	}

	var decodedJSON interface{}

	if err := json.Unmarshal([]byte(jsonString), &decodedJSON); err != nil {
		return nil, fmt.Errorf("JSON parsing error: %v in %s", err, jsonString.GoString())
	}

	v, err := convertJSONToStarlark(decodedJSON)
	if err != nil {
		return nil, fmt.Errorf("error converting JSON to Starlark: %v in %s", err, jsonString.GoString())
	}
	return v, nil
}

func (s *tiltfileState) readJson(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.String
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	localPath, err := s.localPathFromSkylarkValue(path)
	if err != nil {
		return nil, fmt.Errorf("Argument 0 (path): %v", err)
	}

	contents, err := s.readFile(localPath)
	if err != nil {
		return nil, err
	}

	var decodedJSON interface{}

	if err := json.Unmarshal(contents, &decodedJSON); err != nil {
		return nil, fmt.Errorf("JSON parsing error: %v in %s", err, path.GoString())
	}

	v, err := convertJSONToStarlark(decodedJSON)
	if err != nil {
		return nil, fmt.Errorf("error converting JSON to Starlark: %v in %s", err, path.GoString())
	}
	return v, nil
}

func convertJSONToStarlark(j interface{}) (starlark.Value, error) {
	switch j := j.(type) {
	case string:
		return starlark.String(j), nil
	case float64:
		return starlark.Float(j), nil
	case []interface{}:
		listOfValues := []starlark.Value{}

		for _, v := range j {
			convertedValue, err := convertJSONToStarlark(v)
			if err != nil {
				return nil, err
			}
			listOfValues = append(listOfValues, convertedValue)
		}

		return starlark.NewList(listOfValues), nil
	case map[string]interface{}:
		mapOfValues := &starlark.Dict{}

		for k, v := range j {
			convertedValue, err := convertJSONToStarlark(v)
			if err != nil {
				return nil, err
			}

			err = mapOfValues.SetKey(starlark.String(k), convertedValue)
			if err != nil {
				return nil, err
			}
		}

		return mapOfValues, nil
	case nil:
		return starlark.None, nil
	}

	return nil, errors.New(fmt.Sprintf("Unable to convert json to starlark value, unexpected type %T", j))
}

package tiltfile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/sliceutils"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/kustomize"
	"github.com/windmilleng/tilt/internal/ospath"
)

const localLogPrefix = " → "

type gitRepo struct {
	basePath string
}

func (s *tiltfileState) newGitRepo(path string) (*gitRepo, error) {
	absPath := s.absPath(path)
	_, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("Reading paths %s: %v", path, err)
	}

	if _, err := os.Stat(filepath.Join(absPath, ".git")); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s isn't a valid git repo: it doesn't have a .git/ directory", absPath)
	}

	return &gitRepo{basePath: absPath}, nil
}

func (s *tiltfileState) localGitRepo(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "paths", &path)
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
	return gr.basePath != ""
}

func (*gitRepo) Hash() (uint32, error) {
	return 0, errors.New("unhashable type: gitRepo")
}

func (gr *gitRepo) Attr(name string) (starlark.Value, error) {
	switch name {
	case "paths":
		return starlark.NewBuiltin(name, gr.path), nil
	default:
		return nil, nil
	}

}

func (gr *gitRepo) AttrNames() []string {
	return []string{"paths"}
}

func (gr *gitRepo) path(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "paths", &path)
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

func (s *tiltfileState) watchFile(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.Value
	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "paths", &path)
	if err != nil {
		return nil, err
	}

	p, err := s.localPathFromSkylarkValue(path)
	if err != nil {
		return nil, fmt.Errorf("invalid type for paths: %v", err)
	}

	s.recordConfigFile(p.path)

	return starlark.None, nil
}

func (s *tiltfileState) skylarkReadFile(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.Value
	defaultReturn := ""
	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "paths", &path, "default?", &defaultReturn)
	if err != nil {
		return nil, err
	}

	p, err := s.localPathFromSkylarkValue(path)
	if err != nil {
		return nil, fmt.Errorf("invalid type for paths: %v", err)
	}

	bs, err := s.readFile(p)
	if os.IsNotExist(err) {
		bs = []byte(defaultReturn)
	} else if err != nil {
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

	s.logger.Infof("local: %s", command)
	out, err := s.execLocalCmd(exec.Command("sh", "-c", command), true)
	if err != nil {
		return nil, err
	}

	return newBlob(out, fmt.Sprintf("local: %s", command)), nil
}

func (s *tiltfileState) execLocalCmd(c *exec.Cmd, logOutput bool) (string, error) {
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	// TODO(nick): Should this also inject any docker.Env overrides?
	c.Dir = filepath.Dir(s.filename.path)
	c.Stdout = stdout
	c.Stderr = stderr

	if logOutput {
		logOutput := NewMutexWriter(logger.NewPrefixedWriter(localLogPrefix, s.logger.Writer(logger.InfoLvl)))
		c.Stdout = io.MultiWriter(stdout, logOutput)
		c.Stderr = io.MultiWriter(stderr, logOutput)
	}

	err := c.Run()
	if err != nil {
		// If we already logged the output, we don't need to log it again.
		if logOutput {
			return "", fmt.Errorf("command %q failed.\nerror: %v", c.Args, err)
		}

		errorMessage := fmt.Sprintf("command %q failed.\nerror: %v\nstdout: %q", c.Args, err, stdout.String())
		exitError, ok := err.(*exec.ExitError)
		if ok {
			errorMessage += fmt.Sprintf("\nstderr: %q", string(exitError.Stderr))
		}
		return "", errors.New(errorMessage)
	}

	if stdout.Len() == 0 && stderr.Len() == 0 {
		s.logger.Infof("%s[no output]", localLogPrefix)
	}

	return stdout.String(), nil
}

func (s *tiltfileState) kustomize(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.Value
	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "paths", &path)
	if err != nil {
		return nil, err
	}

	kustomizePath, err := s.localPathFromSkylarkValue(path)
	if err != nil {
		return nil, fmt.Errorf("Argument 0 (paths): %v", err)
	}

	yaml, err := s.execLocalCmd(exec.Command("kustomize", "build", kustomizePath.String()), false)
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
	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "paths", &path)
	if err != nil {
		return nil, err
	}

	localPath, err := s.localPathFromSkylarkValue(path)
	if err != nil {
		return nil, fmt.Errorf("Argument 0 (paths): %v", err)
	}

	info, err := os.Stat(localPath.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Could not read Helm chart directory %q: does not exist", localPath.path)
		}
		return nil, fmt.Errorf("Could not read Helm chart directory %q: %v", localPath.path, err)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("helm() may only be called on directories with Chart.yaml: %q", localPath.path)
	}

	cmd := []string{"helm", "template", localPath.path}
	yaml, err := s.execLocalCmd(exec.Command(cmd[0], cmd[1:]...), false)
	if err != nil {
		return nil, err
	}

	s.recordConfigFile(localPath.path)

	return newBlob(filterHelmTestYAML(string(yaml)), fmt.Sprintf("helm: %s", localPath.path)), nil
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

	localPath, err := s.localPathFromSkylarkValue(dir)
	if err != nil {
		return nil, fmt.Errorf("Argument 0 (paths): %v", err)
	}
	s.recordConfigFile(localPath.path)
	var files []string
	err = filepath.Walk(localPath.path, func(path string, info os.FileInfo, err error) error {
		if path == localPath.path {
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

func (s *tiltfileState) readYaml(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.String
	var defaultValue starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "paths", &path, "default?", &defaultValue); err != nil {
		return nil, err
	}

	localPath, err := s.localPathFromSkylarkValue(path)
	if err != nil {
		return nil, fmt.Errorf("Argument 0 (paths): %v", err)
	}

	contents, err := s.readFile(localPath)
	if err != nil {
		// Return the default value if the file doesn't exist AND a default value was given
		if os.IsNotExist(err) && defaultValue != nil {
			return defaultValue, nil
		}
		return nil, err
	}

	var decodedYAML interface{}
	err = yaml.Unmarshal(contents, &decodedYAML)
	if err != nil {
		return nil, fmt.Errorf("error parsing YAML: %v in %s", err, path.GoString())
	}

	v, err := convertStructuredDataToStarlark(decodedYAML)
	if err != nil {
		return nil, fmt.Errorf("error converting YAML to Starlark: %v in %s", err, path.GoString())
	}
	return v, nil
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

	v, err := convertStructuredDataToStarlark(decodedJSON)
	if err != nil {
		return nil, fmt.Errorf("error converting JSON to Starlark: %v in %s", err, jsonString.GoString())
	}
	return v, nil
}

func (s *tiltfileState) readJson(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.String
	var defaultValue starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "paths", &path, "default?", &defaultValue); err != nil {
		return nil, err
	}

	localPath, err := s.localPathFromSkylarkValue(path)
	if err != nil {
		return nil, fmt.Errorf("Argument 0 (paths): %v", err)
	}

	contents, err := s.readFile(localPath)
	if err != nil {
		// Return the default value if the file doesn't exist AND a default value was given
		if os.IsNotExist(err) && defaultValue != nil {
			return defaultValue, nil
		}
		return nil, err
	}

	var decodedJSON interface{}

	if err := json.Unmarshal(contents, &decodedJSON); err != nil {
		return nil, fmt.Errorf("JSON parsing error: %v in %s", err, path.GoString())
	}

	v, err := convertStructuredDataToStarlark(decodedJSON)
	if err != nil {
		return nil, fmt.Errorf("error converting JSON to Starlark: %v in %s", err, path.GoString())
	}
	return v, nil
}

func convertStructuredDataToStarlark(j interface{}) (starlark.Value, error) {
	switch j := j.(type) {
	case bool:
		return starlark.Bool(j), nil
	case string:
		return starlark.String(j), nil
	case float64:
		return starlark.Float(j), nil
	case []interface{}:
		listOfValues := []starlark.Value{}

		for _, v := range j {
			convertedValue, err := convertStructuredDataToStarlark(v)
			if err != nil {
				return nil, err
			}
			listOfValues = append(listOfValues, convertedValue)
		}

		return starlark.NewList(listOfValues), nil
	case map[string]interface{}:
		mapOfValues := &starlark.Dict{}

		for k, v := range j {
			convertedValue, err := convertStructuredDataToStarlark(v)
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

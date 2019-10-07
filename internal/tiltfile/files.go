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

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/sliceutils"
	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/pkg/logger"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/kustomize"
)

const localLogPrefix = " → "

type gitRepo struct {
	basePath string
}

func (s *tiltfileState) newGitRepo(t *starlark.Thread, path string) (*gitRepo, error) {
	absPath := s.absPath(t, path)
	_, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("Reading paths %s: %v", absPath, err)
	}

	if _, err := os.Stat(filepath.Join(absPath, ".git")); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s isn't a valid git repo: it doesn't have a .git/ directory", absPath)
	}

	return &gitRepo{basePath: absPath}, nil
}

func (s *tiltfileState) localGitRepo(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	err := s.unpackArgs(fn.Name(), args, kwargs, "paths", &path)
	if err != nil {
		return nil, err
	}

	return s.newGitRepo(thread, path)
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
	err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "paths", &path)
	if err != nil {
		return nil, err
	}

	return starlark.String(gr.makeLocalPath(path)), nil
}

func (gr *gitRepo) makeLocalPath(path string) string {
	return filepath.Join(gr.basePath, path)
}

func (s *tiltfileState) absPathFromStarlarkValue(thread *starlark.Thread, v starlark.Value) (string, error) {
	switch v := v.(type) {
	case *gitRepo:
		return v.makeLocalPath("."), nil
	case starlark.String:
		return s.absPath(thread, string(v)), nil
	default:
		return "", fmt.Errorf("expected gitRepo | string. Actual type: %T", v)
	}
}

// When running the Tilt demo, the current working directory is arbitrary.
// So we want to resolve paths relative to the dir where the Tiltfile lives,
// not relative to the working directory.
func (s *tiltfileState) absPath(t *starlark.Thread, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(s.absWorkingDir(t), path)
}

func (s *tiltfileState) absWorkingDir(t *starlark.Thread) string {
	return filepath.Dir(s.currentTiltfilePath(t))
}

func (s *tiltfileState) recordConfigFile(f string) {
	s.configFiles = sliceutils.AppendWithoutDupes(s.configFiles, f)
}

func (s *tiltfileState) readFile(p string) ([]byte, error) {
	s.recordConfigFile(p)
	return ioutil.ReadFile(p)
}

func (s *tiltfileState) watchFile(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.Value
	err := s.unpackArgs(fn.Name(), args, kwargs, "paths", &path)
	if err != nil {
		return nil, err
	}

	p, err := s.absPathFromStarlarkValue(thread, path)
	if err != nil {
		return nil, fmt.Errorf("invalid type for paths: %v", err)
	}

	s.recordConfigFile(p)

	return starlark.None, nil
}

func (s *tiltfileState) skylarkReadFile(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.Value
	defaultReturn := ""
	err := s.unpackArgs(fn.Name(), args, kwargs, "paths", &path, "default?", &defaultReturn)
	if err != nil {
		return nil, err
	}

	p, err := s.absPathFromStarlarkValue(thread, path)
	if err != nil {
		return nil, fmt.Errorf("invalid type for paths: %v", err)
	}

	bs, err := s.readFile(p)
	if os.IsNotExist(err) {
		bs = []byte(defaultReturn)
	} else if err != nil {
		return nil, err
	}

	return newBlob(string(bs), fmt.Sprintf("file: %s", p)), nil
}

func (s *tiltfileState) local(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var command string
	err := s.unpackArgs(fn.Name(), args, kwargs, "command", &command)
	if err != nil {
		return nil, err
	}

	s.logger.Infof("local: %s", command)
	out, err := s.execLocalCmd(thread, exec.Command("sh", "-c", command), true)
	if err != nil {
		return nil, err
	}

	return newBlob(out, fmt.Sprintf("local: %s", command)), nil
}

func (s *tiltfileState) execLocalCmd(t *starlark.Thread, c *exec.Cmd, logOutput bool) (string, error) {
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	// TODO(nick): Should this also inject any docker.Env overrides?
	c.Dir = s.absWorkingDir(t)
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
	err := s.unpackArgs(fn.Name(), args, kwargs, "paths", &path)
	if err != nil {
		return nil, err
	}

	kustomizePath, err := s.absPathFromStarlarkValue(thread, path)
	if err != nil {
		return nil, fmt.Errorf("Argument 0 (paths): %v", err)
	}

	yaml, err := s.execLocalCmd(thread, exec.Command("kustomize", "build", kustomizePath), false)
	if err != nil {
		return nil, err
	}
	deps, err := kustomize.Deps(kustomizePath)
	if err != nil {
		return nil, fmt.Errorf("internal error: %v", err)
	}
	for _, d := range deps {
		s.recordConfigFile(d)
	}

	return newBlob(yaml, fmt.Sprintf("kustomize: %s", kustomizePath)), nil
}

func (s *tiltfileState) helm(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.Value
	var name string
	var namespace string
	var valueFilesV starlark.Value
	err := s.unpackArgs(fn.Name(), args, kwargs,
		"paths", &path,
		"name?", &name,
		"namespace?", &namespace,
		"values?", &valueFilesV)
	if err != nil {
		return nil, err
	}

	localPath, err := s.absPathFromStarlarkValue(thread, path)
	if err != nil {
		return nil, fmt.Errorf("Argument 0 (paths): %v", err)
	}

	valueFiles, ok := AsStringOrStringList(valueFilesV)
	if !ok {
		return nil, fmt.Errorf("Argument 'values' must be string or list of strings. Actual: %T",
			valueFilesV)
	}

	info, err := os.Stat(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Could not read Helm chart directory %q: does not exist", localPath)
		}
		return nil, fmt.Errorf("Could not read Helm chart directory %q: %v", localPath, err)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("helm() may only be called on directories with Chart.yaml: %q", localPath)
	}

	cmd := []string{"helm", "template", localPath}
	if name != "" {
		cmd = append(cmd, "--name", name)
	}
	if namespace != "" {
		cmd = append(cmd, "--namespace", namespace)
	}
	for _, valueFile := range valueFiles {
		cmd = append(cmd, "--values", valueFile)
		s.recordConfigFile(s.absPath(thread, valueFile))
	}

	stdout, err := s.execLocalCmd(thread, exec.Command(cmd[0], cmd[1:]...), false)
	if err != nil {
		return nil, err
	}

	s.recordConfigFile(localPath)

	yaml := filterHelmTestYAML(string(stdout))

	if namespace != "" {
		// helm template --namespace doesn't inject the namespace,
		// so we have to do that ourselves :\
		// https://github.com/helm/helm/issues/5465
		entities, err := k8s.ParseYAMLFromString(yaml)
		if err != nil {
			return nil, err
		}

		for i, e := range entities {
			entities[i] = e.WithNamespace(namespace)
		}
		yaml, err = k8s.SerializeSpecYAML(entities)
		if err != nil {
			return nil, err
		}
	}

	return newBlob(yaml, fmt.Sprintf("helm: %s", localPath)), nil
}

func (s *tiltfileState) blob(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var input starlark.String
	err := s.unpackArgs(fn.Name(), args, kwargs, "input", &input)
	if err != nil {
		return nil, err
	}

	return newBlob(input.GoString(), "Tiltfile blob() call"), nil
}

func (s *tiltfileState) listdir(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var dir starlark.String
	var recursive bool
	err := s.unpackArgs(fn.Name(), args, kwargs, "dir", &dir, "recursive?", &recursive)
	if err != nil {
		return nil, err
	}

	localPath, err := s.absPathFromStarlarkValue(thread, dir)
	if err != nil {
		return nil, fmt.Errorf("Argument 0 (paths): %v", err)
	}
	s.recordConfigFile(localPath)
	var files []string
	err = filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if path == localPath {
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
	if err := s.unpackArgs(fn.Name(), args, kwargs, "paths", &path, "default?", &defaultValue); err != nil {
		return nil, err
	}

	localPath, err := s.absPathFromStarlarkValue(thread, path)
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
	if err := s.unpackArgs(fn.Name(), args, kwargs, "json", &jsonString); err != nil {
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
	if err := s.unpackArgs(fn.Name(), args, kwargs, "paths", &path, "default?", &defaultValue); err != nil {
		return nil, err
	}

	localPath, err := s.absPathFromStarlarkValue(thread, path)
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

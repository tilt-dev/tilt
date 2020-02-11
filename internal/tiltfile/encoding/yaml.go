package encoding

import (
	"fmt"
	"os"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	tiltfile_io "github.com/windmilleng/tilt/internal/tiltfile/io"
	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/internal/tiltfile/value"
)

// reads yaml from a file
func readYAML(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.String
	var defaultValue starlark.Value
	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "paths", &path, "default?", &defaultValue); err != nil {
		return nil, err
	}

	localPath, err := value.ValueToAbsPath(thread, path)
	if err != nil {
		return nil, fmt.Errorf("Argument 0 (paths): %v", err)
	}

	contents, err := tiltfile_io.ReadFile(thread, localPath)
	if err != nil {
		// Return the default value if the file doesn't exist AND a default value was given
		if os.IsNotExist(err) && defaultValue != nil {
			return defaultValue, nil
		}
		return nil, err
	}

	return yamlStringToStarlark(string(contents), path.GoString())
}

// reads yaml from a string
func decodeYAML(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var contents starlark.Value
	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "yaml", &contents); err != nil {
		return nil, err
	}

	s, ok := value.AsString(contents)
	if !ok {
		return nil, fmt.Errorf("%s arg must be a string or blob. got %s", fn.Name(), contents.Type())
	}

	return yamlStringToStarlark(s, "")
}

func yamlStringToStarlark(s string, source string) (starlark.Value, error) {
	var decodedYAML interface{}
	err := yaml.Unmarshal([]byte(s), &decodedYAML)
	if err != nil {
		errmsg := "error parsing YAML"
		if source != "" {
			errmsg += fmt.Sprintf(" from %s", source)
		}
		return nil, errors.Wrap(err, errmsg)
	}

	v, err := convertStructuredDataToStarlark(decodedYAML)
	if err != nil {
		errmsg := "error converting YAML to Starlark"
		if source != "" {
			errmsg += fmt.Sprintf(" from %s", source)
		}
		return nil, errors.Wrap(err, errmsg)
	}
	return v, nil
}

// dumps yaml to a string
func encodeYAML(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var obj starlark.Value
	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "obj", &obj); err != nil {
		return nil, err
	}

	ret, err := starlarkToYAMLString(obj)
	if err != nil {
		return nil, err
	}

	return tiltfile_io.NewBlob(ret, "encode_yaml"), nil
}

func starlarkToYAMLString(obj starlark.Value) (string, error) {
	v, err := convertStarlarkToStructuredData(obj)
	if err != nil {
		return "", errors.Wrap(err, "error converting object from starlark")
	}

	b, err := yaml.Marshal(v)
	if err != nil {
		return "", errors.Wrap(err, "error serializing object to yaml")
	}

	return string(b), nil
}

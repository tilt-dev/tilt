package encoding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	tiltfile_io "github.com/windmilleng/tilt/internal/tiltfile/io"
	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/internal/tiltfile/value"
)

// reads json from a file
func readJSON(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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

	return jsonStringToStarlark(string(contents), path.GoString())
}

// reads json from a string
func decodeJSON(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var contents starlark.Value
	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "json", &contents); err != nil {
		return nil, err
	}

	s, ok := value.AsString(contents)
	if !ok {
		return nil, fmt.Errorf("%s arg must be a string or blob. got %s", fn.Name(), contents.Type())
	}

	return jsonStringToStarlark(s, "")
}

func jsonStringToStarlark(s string, source string) (starlark.Value, error) {
	var decodedJSON interface{}
	if err := json.Unmarshal([]byte(s), &decodedJSON); err != nil {
		errmsg := "error parsing JSON"
		if source != "" {
			errmsg += fmt.Sprintf(" from %s", source)
		}
		return nil, errors.Wrap(err, errmsg)
	}

	v, err := convertStructuredDataToStarlark(decodedJSON)
	if err != nil {
		errmsg := "error converting JSON to Starlark"
		if source != "" {
			errmsg += fmt.Sprintf(" from %s", source)
		}
		return nil, errors.Wrap(err, errmsg)
	}
	return v, nil
}

func encodeJSON(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var obj starlark.Value
	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "obj", &obj); err != nil {
		return nil, err
	}

	ret, err := starlarkToJSONString(obj)
	if err != nil {
		return nil, err
	}

	return starlark.String(ret), nil
}

func starlarkToJSONString(obj starlark.Value) (string, error) {
	v, err := convertStarlarkToStructuredData(obj)
	if err != nil {
		return "", errors.Wrap(err, "error converting object from starlark")
	}

	w := bytes.Buffer{}
	e := json.NewEncoder(&w)
	e.SetIndent("", "  ")
	err = e.Encode(v)
	if err != nil {
		return "", errors.Wrap(err, "error serializing object to json")
	}

	return w.String(), nil
}

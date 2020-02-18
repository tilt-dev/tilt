package encoding

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"

	tiltfile_io "github.com/windmilleng/tilt/internal/tiltfile/io"
	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/internal/tiltfile/value"
)

// reads yaml from a file
func readYAMLStream(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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

	return yamlStreamToStarlark(string(contents), path.GoString())
}

func readYAML(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	v, err := readYAMLStream(thread, fn, args, kwargs)
	if err != nil {
		return nil, err
	}
	l, ok := v.(*starlark.List)
	if !ok {
		return nil, fmt.Errorf("internal error: expected readYAMLStream to return a %T, but it returned a %T", starlark.List{}, l)
	}
	if l.Len() != 1 {
		return nil, fmt.Errorf("expected a yaml document but found a yaml stream (documents separated by `---`). use %s instead to read a yaml stream", readYAMLStreamN)
	}
	return l.Index(0), nil
}

// reads yaml from a string
func decodeYAMLStream(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var contents starlark.Value
	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "yaml", &contents); err != nil {
		return nil, err
	}

	s, ok := value.AsString(contents)
	if !ok {
		return nil, fmt.Errorf("%s arg must be a string or blob. got %s", fn.Name(), contents.Type())
	}

	return yamlStreamToStarlark(s, "")
}

func decodeYAML(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	v, err := decodeYAMLStream(thread, fn, args, kwargs)
	if err != nil {
		return nil, err
	}
	l, ok := v.(*starlark.List)
	if !ok {
		return nil, fmt.Errorf("internal error: expected decodeYAMLStream to return a %T, but it returned a %T", starlark.List{}, l)
	}
	if l.Len() != 1 {
		return nil, fmt.Errorf("expected a yaml document but found a yaml stream (documents separated by `---`). use %s instead to decode a yaml stream", decodeYAMLStreamN)
	}
	return l.Index(0), nil
}

func yamlStreamToStarlark(s string, source string) (*starlark.List, error) {
	var ret []starlark.Value
	var decodedYAML interface{}
	d := k8syaml.NewYAMLToJSONDecoder(strings.NewReader(s))
	for {
		err := d.Decode(&decodedYAML)
		if err == io.EOF {
			break
		}

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

		ret = append(ret, v)
	}

	return starlark.NewList(ret), nil
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

func encodeYAMLStream(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var objs *starlark.List
	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "objs", &objs); err != nil {
		return nil, err
	}

	var yamlDocs []string

	it := objs.Iterate()
	defer it.Done()
	var v starlark.Value
	for it.Next(&v) {
		s, err := starlarkToYAMLString(v)
		if err != nil {
			return nil, err
		}
		yamlDocs = append(yamlDocs, s)
	}

	return tiltfile_io.NewBlob(strings.Join(yamlDocs, "---\n"), "encode_yaml_stream"), nil
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

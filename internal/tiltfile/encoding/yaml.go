package encoding

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"

	tiltfile_io "github.com/tilt-dev/tilt/internal/tiltfile/io"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
)

// takes a list of objects that came from deserializing a potential starlark stream
// ensures there's only one, and returns it
func singleYAMLDoc(l *starlark.List) (starlark.Value, error) {
	switch l.Len() {
	case 0:
		// if there are zero documents in the stream, that's actually an empty yaml document, which is a yaml
		// document with a scalar value of NULL
		return starlark.None, nil
	case 1:
		return l.Index(0), nil
	default:
		return nil, fmt.Errorf("expected a yaml document but found a yaml stream (documents separated by `---`). use %s instead to decode a yaml stream", decodeYAMLStreamN)
	}
}

func readYAMLStreamAsStarlarkList(thread *starlark.Thread, path starlark.String, defaultValue *starlark.List) (*starlark.List, error) {
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

func readYAMLStream(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.String
	var defaultValue *starlark.List
	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "paths", &path, "default?", &defaultValue); err != nil {
		return nil, err
	}

	return readYAMLStreamAsStarlarkList(thread, path, defaultValue)
}

func readYAML(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.String
	var defaultValue starlark.Value
	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "paths", &path, "default?", &defaultValue); err != nil {
		return nil, err
	}

	var defaultValueList *starlark.List
	if defaultValue != nil {
		defaultValueList = starlark.NewList([]starlark.Value{defaultValue})
	}

	l, err := readYAMLStreamAsStarlarkList(thread, path, defaultValueList)
	if err != nil {
		return nil, err
	}

	return singleYAMLDoc(l)
}

func decodeYAMLStreamAsStarlarkList(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (*starlark.List, error) {
	var contents value.Stringable
	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "yaml", &contents); err != nil {
		return nil, err
	}

	return yamlStreamToStarlark(contents.Value, "")
}

func decodeYAMLStream(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return decodeYAMLStreamAsStarlarkList(thread, fn, args, kwargs)
}

func decodeYAML(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	l, err := decodeYAMLStreamAsStarlarkList(thread, fn, args, kwargs)
	if err != nil {
		return nil, err
	}

	return singleYAMLDoc(l)
}

func yamlStreamToStarlark(s string, source string) (*starlark.List, error) {
	var ret []starlark.Value
	r := k8syaml.NewYAMLReader(bufio.NewReader(strings.NewReader(s)))

	for {
		bytes, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, wrapError(err, "error reading YAML stream", source)
		}

		var decodedYAML interface{}
		err = k8syaml.Unmarshal(bytes, &decodedYAML)
		if err != nil {
			return nil, wrapError(err, "error parsing YAML", source)
		}

		v, err := ConvertStructuredDataToStarlark(decodedYAML)
		if err != nil {
			return nil, wrapError(err, "error converting YAML to Starlark", source)
		}
		if v == starlark.None {
			continue // ignore empty entries
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

func wrapError(err error, errmsg string, source string) error {
	if source != "" {
		errmsg += fmt.Sprintf(" from %s", source)
	}
	return errors.Wrap(err, errmsg)
}

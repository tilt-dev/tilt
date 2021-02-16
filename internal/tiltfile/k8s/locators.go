package k8s

import (
	"fmt"

	"go.starlark.net/starlark"

	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
)

// Deserializing locators from starlark values.
type JSONPathImageLocatorListSpec struct {
	Specs []JSONPathImageLocatorSpec
}

func (s JSONPathImageLocatorListSpec) IsEmpty() bool {
	return len(s.Specs) == 0
}

func (s *JSONPathImageLocatorListSpec) Unpack(v starlark.Value) error {
	list := value.ValueOrSequenceToSlice(v)
	for _, item := range list {
		spec := JSONPathImageLocatorSpec{}
		err := spec.Unpack(item)
		if err != nil {
			return err
		}
		s.Specs = append(s.Specs, spec)
	}
	return nil
}

func (s JSONPathImageLocatorListSpec) ToImageLocators(selector k8s.ObjectSelector) ([]k8s.ImageLocator, error) {
	result := []k8s.ImageLocator{}
	for _, spec := range s.Specs {
		locator, err := spec.ToImageLocator(selector)
		if err != nil {
			return nil, err
		}
		result = append(result, locator)
	}
	return result, nil
}

type JSONPathImageLocatorSpec struct {
	jsonPath string
}

func (s *JSONPathImageLocatorSpec) Unpack(v starlark.Value) error {
	var ok bool
	s.jsonPath, ok = starlark.AsString(v)
	if !ok {
		return fmt.Errorf("Expected string, got: %s", v)
	}
	return nil
}

func (s JSONPathImageLocatorSpec) ToImageLocator(selector k8s.ObjectSelector) (k8s.ImageLocator, error) {
	return k8s.NewJSONPathImageLocator(selector, s.jsonPath)
}

type JSONPathImageObjectLocatorSpec struct {
	jsonPath  string
	repoField string
	tagField  string
}

func (s JSONPathImageObjectLocatorSpec) IsEmpty() bool {
	return s == JSONPathImageObjectLocatorSpec{}
}

func (s *JSONPathImageObjectLocatorSpec) Unpack(v starlark.Value) error {
	d, ok := v.(*starlark.Dict)
	if !ok {
		return fmt.Errorf("Expected dict of the form {'json_path': str, 'repo_field': str, 'tag_field': str}. Actual: %s", v)
	}

	values, err := validateStringDict(d, []string{"json_path", "repo_field", "tag_field"})
	if err != nil {
		return errors.Wrap(err, "Expected dict of the form {'json_path': str, 'repo_field': str, 'tag_field': str}")
	}

	s.jsonPath, s.repoField, s.tagField = values[0], values[1], values[2]
	return nil
}

func (s JSONPathImageObjectLocatorSpec) ToImageLocator(selector k8s.ObjectSelector) (k8s.ImageLocator, error) {
	return k8s.NewJSONPathImageObjectLocator(selector, s.jsonPath, s.repoField, s.tagField)
}

func validateStringDict(d *starlark.Dict, expectedFields []string) ([]string, error) {
	indices := map[string]int{}
	result := make([]string, len(expectedFields))
	for i, f := range expectedFields {
		indices[f] = i
	}

	for _, item := range d.Items() {
		key, val := item[0], item[1]
		keyString, ok := starlark.AsString(key)
		if !ok {
			return nil, fmt.Errorf("Unexpected key: %s", key)
		}

		index, ok := indices[keyString]
		if !ok {
			return nil, fmt.Errorf("Unexpected key: %s", key)
		}

		valString, ok := starlark.AsString(val)
		if !ok {
			return nil, fmt.Errorf("Expected string at key %q. Got: %s", key, val.Type())
		}

		result[index] = valString
	}

	if len(d.Items()) != len(expectedFields) {
		return nil, fmt.Errorf("Missing keys. Actual keys: %s", d.Keys())
	}
	return result, nil
}

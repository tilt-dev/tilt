package config

import (
	"encoding/json"
	"flag"
	"fmt"

	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/encoding"
)

type objectSetting struct {
	value starlark.Value
	isSet bool
}

var _ configValue = &objectSetting{}
var _ flag.Value = &objectSetting{}

func (s *objectSetting) starlark() starlark.Value {
	return s.value
}

func (s *objectSetting) IsSet() bool {
	return s.isSet
}

func (s *objectSetting) Type() string {
	return "object"
}

func (s *objectSetting) setFromInterface(i interface{}) error {
	if i == nil {
		return nil
	}
	v, err := encoding.ConvertStructuredDataToStarlark(i)
	if err != nil {
		return err
	}

	s.value = v
	s.isSet = true

	return nil
}

func (s *objectSetting) Set(str string) error {
	if s.isSet {
		return fmt.Errorf("object settings can only be specified once. multiple values found (last value: %s)", s.value)
	}

	var decoded interface{}
	err := json.Unmarshal([]byte(str), &decoded)
	if err != nil {
		return fmt.Errorf("decoding JSON, got %q: %v", str, err)
	}

	v, err := encoding.ConvertStructuredDataToStarlark(decoded)
	if err != nil {
		return err
	}

	s.value = v
	s.isSet = true
	return nil
}

func (s *objectSetting) String() string {
	if !s.isSet {
		return "None"
	}
	return s.value.String()
}

package config

import (
	"flag"
	"fmt"

	"go.starlark.net/starlark"
)

type stringSetting struct {
	value string
	isSet bool
}

var _ configValue = &stringSetting{}
var _ flag.Value = &stringSetting{}

func (s *stringSetting) starlark() starlark.Value {
	return starlark.String(s.value)
}

func (s *stringSetting) IsSet() bool {
	return s.isSet
}

func (s *stringSetting) setFromInterface(i interface{}) error {
	if i == nil {
		return nil
	}
	v, ok := i.(string)
	if !ok {
		return fmt.Errorf("expected %T, found %T", s.value, i)
	}

	s.value = v
	s.isSet = true

	return nil
}

func (s *stringSetting) Set(v string) error {
	if s.isSet {
		return fmt.Errorf("string settings can only be specified once. multiple values found (last value: %s", v)
	}

	s.value = v
	s.isSet = true
	return nil
}

func (s *stringSetting) String() string {
	return s.value
}

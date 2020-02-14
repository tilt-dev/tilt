package config

import (
	"flag"
	"fmt"
	"strconv"

	"go.starlark.net/starlark"
)

type boolSetting struct {
	value bool
	isSet bool
}

var _ configValue = &boolSetting{}
var _ flag.Value = &boolSetting{}

func (s *boolSetting) starlark() starlark.Value {
	return starlark.Bool(s.value)
}

func (s *boolSetting) IsSet() bool {
	return s.isSet
}

func (s *boolSetting) setFromInterface(i interface{}) error {
	if i == nil {
		return nil
	}
	v, ok := i.(bool)
	if !ok {
		return fmt.Errorf("expected %T, found %T", s.value, i)
	}

	s.value = v
	s.isSet = true

	return nil
}

func (s *boolSetting) Set(v string) error {
	if s.isSet {
		return fmt.Errorf("bool settings can only be specified once. multiple values found (last value: %s)", v)
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return err
	}
	s.value = b
	s.isSet = true
	return nil
}

func (s *boolSetting) String() string {
	return strconv.FormatBool(s.value)
}

func (s *boolSetting) IsBoolFlag() bool {
	return true
}

package config

import (
	"flag"
	"fmt"
	"strings"

	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/value"
)

type stringList struct {
	Values []string
	isSet  bool
}

var _ configValue = &stringList{}
var _ flag.Value = &stringList{}

func (s *stringList) starlark() starlark.Value {
	return value.StringSliceToList(s.Values)
}

func (s *stringList) IsSet() bool {
	return s.isSet
}

func (s *stringList) setFromInterface(i interface{}) error {
	if i == nil {
		s.Values = nil
		return nil
	}
	is, ok := i.([]interface{})
	if !ok {
		return fmt.Errorf("expected array")
	}
	s.Values = nil
	for _, elem := range is {
		str, ok := elem.(string)
		if !ok {
			return fmt.Errorf("expected string, got %T", elem)
		}
		s.Values = append(s.Values, str)
	}

	s.isSet = true
	return nil
}

func (s *stringList) Set(v string) error {
	s.Values = append(s.Values, v)
	s.isSet = true
	return nil
}

func (s *stringList) String() string {
	return strings.Join(s.Values, ",")
}

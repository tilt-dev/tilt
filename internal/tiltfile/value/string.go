package value

import (
	"fmt"

	"go.starlark.net/starlark"
)

type Stringable struct {
	Value string
}

func (s *Stringable) Unpack(v starlark.Value) error {
	str, ok := AsString(v)
	if !ok {
		return fmt.Errorf("Value should be convertible to string, but is type %s", v.Type())
	}
	s.Value = str
	return nil
}

type ImplicitStringer interface {
	ImplicitString() string
}

// Wrapper around starlark.AsString
func AsString(x starlark.Value) (string, bool) {
	is, ok := x.(ImplicitStringer)
	if ok {
		return is.ImplicitString(), true
	}
	return starlark.AsString(x)
}

type StringList []string

var _ starlark.Unpacker = &StringList{}

// Unpack an argument that can be expressed as a list or tuple of strings.
func (s *StringList) Unpack(v starlark.Value) error {
	*s = nil
	if v == nil {
		return nil
	}

	var iter starlark.Iterator
	switch x := v.(type) {
	case *starlark.List:
		iter = x.Iterate()
	case starlark.Tuple:
		iter = x.Iterate()
	case starlark.NoneType:
		return nil
	default:
		return fmt.Errorf("value should be a List or Tuple of strings, but is of type %s", v.Type())
	}

	defer iter.Done()
	var item starlark.Value
	for iter.Next(&item) {
		sv, ok := AsString(item)
		if !ok {
			return fmt.Errorf("value should contain only strings, but element %q was of type %s", item.String(), item.Type())
		}
		*s = append(*s, sv)
	}

	return nil
}

type StringOrStringList struct {
	Values []string
}

var _ starlark.Unpacker = &StringOrStringList{}

// Unpack an argument that can either be expressed as
// a string or as a list of strings.
func (s *StringOrStringList) Unpack(v starlark.Value) error {
	s.Values = nil
	if v == nil {
		return nil
	}

	vs, ok := AsString(v)
	if ok {
		s.Values = []string{vs}
		return nil
	}

	var iter starlark.Iterator
	switch x := v.(type) {
	case *starlark.List:
		iter = x.Iterate()
	case starlark.Tuple:
		iter = x.Iterate()
	case starlark.NoneType:
		return nil
	default:
		return fmt.Errorf("value should be a string or List or Tuple of strings, but is of type %s", v.Type())
	}

	defer iter.Done()
	var item starlark.Value
	for iter.Next(&item) {
		sv, ok := AsString(item)
		if !ok {
			return fmt.Errorf("list should contain only strings, but element %q was of type %s", item.String(), item.Type())
		}
		s.Values = append(s.Values, sv)
	}

	return nil
}

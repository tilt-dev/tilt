package value

import (
	"fmt"

	"go.starlark.net/starlark"
)

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

// Unpack an argument that can either be expressed as
// a string or as a list of strings.
func AsStringOrStringList(x starlark.Value) ([]string, error) {
	if x == nil {
		return []string{}, nil
	}

	s, ok := AsString(x)
	if ok {
		return []string{s}, nil
	}

	list, ok := x.(*starlark.List)
	if !ok {
		return nil, fmt.Errorf("value should be a string or List of strings, but is of type %s", x.Type())
	}

	result := []string{}
	iter := list.Iterate()
	defer iter.Done()
	var item starlark.Value
	for iter.Next(&item) {
		s, ok := AsString(item)
		if !ok {
			return nil, fmt.Errorf("list should contain only strings, but element %q was of type %s", item.String(), item.Type())
		}
		result = append(result, s)
	}
	return result, nil
}

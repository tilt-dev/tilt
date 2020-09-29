package value

import (
	"fmt"

	"go.starlark.net/starlark"
)

type StringStringMap map[string]string

var _ starlark.Unpacker = &StringStringMap{}

func (s *StringStringMap) Unpack(v starlark.Value) error {
	*s = make(map[string]string)
	if v != nil && v != starlark.None {
		d, ok := v.(*starlark.Dict)
		if !ok {
			return fmt.Errorf("expected dict, got %T", v)
		}

		for _, tuple := range d.Items() {
			k, ok := AsString(tuple[0])
			if !ok {
				return fmt.Errorf("key is not a string: %T (%v)", tuple[0], tuple[0])
			}

			v, ok := AsString(tuple[1])
			if !ok {
				return fmt.Errorf("value is not a string: %T (%v)", tuple[1], tuple[1])
			}

			(*s)[k] = v
		}
	}

	return nil
}

func (s *StringStringMap) AsMap() map[string]string {
	return *s
}

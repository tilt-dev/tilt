package value

import (
	"fmt"

	"go.starlark.net/starlark"
)

// Unpack values that could be V or starlark.None
type Optional[V starlark.Value] struct {
	IsSet bool
	Value V
}

func (o *Optional[V]) Unpack(v starlark.Value) error {
	if v == nil {
		return nil
	}
	switch v := v.(type) {
	case starlark.NoneType:
		return nil
	case V:
		o.Value = v
		o.IsSet = true
		return nil
	}

	return fmt.Errorf("expected %T or None, got %T", o.Value, v)
}

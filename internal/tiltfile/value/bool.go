package value

import (
	"fmt"

	"go.starlark.net/starlark"
)

// Unpack values that could be Bool or None
type BoolOrNone struct {
	Value bool
	IsSet bool
}

func (b *BoolOrNone) Unpack(v starlark.Value) error {
	if v == nil {
		return nil
	}
	switch v := v.(type) {
	case starlark.NoneType:
		return nil
	case starlark.Bool:
		b.Value = bool(v)
		b.IsSet = true
		return nil
	}

	return fmt.Errorf("got %s, want bool or None", v.Type())
}

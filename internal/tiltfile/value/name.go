package value

import (
	"fmt"

	"go.starlark.net/starlark"
	"k8s.io/apimachinery/pkg/api/validation/path"
)

// Names in Tilt should be valid Kuberentes API names.
//
// We use the loosest validation rules: valid path segment names.
//
// For discussion, see:
// https://github.com/tilt-dev/tilt/issues/4309
type Name string

func (n *Name) Unpack(v starlark.Value) error {
	str, ok := AsString(v)
	if !ok {
		return fmt.Errorf("Value should be convertible to string, but is type %s", v.Type())
	}

	if errs := path.ValidatePathSegmentName(str, false); len(errs) != 0 {
		return fmt.Errorf("invalid value %q: %v", str, errs[0])
	}

	*n = Name(str)
	return nil
}

func (n Name) String() string {
	return string(n)
}

package value

import (
	"fmt"

	"github.com/docker/distribution/reference"
	"go.starlark.net/starlark"
)

type ImageList []reference.Named

var _ starlark.Unpacker = &ImageList{}

// Unpack an argument that can be expressed as a list or tuple of image references.
func (s *ImageList) Unpack(v starlark.Value) error {
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
		ref, err := reference.ParseNormalizedNamed(sv)
		if err != nil {
			return fmt.Errorf("must be a valid image reference: %v", err)
		}
		*s = append(*s, ref)
	}

	return nil
}

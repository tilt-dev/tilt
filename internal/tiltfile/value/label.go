package value

import (
	"fmt"
	"strings"

	"go.starlark.net/starlark"
	validation "k8s.io/apimachinery/pkg/util/validation"
)

type LabelValue string

func (lv *LabelValue) Unpack(v starlark.Value) error {
	str, ok := AsString(v)
	if !ok {
		return fmt.Errorf("Value should be convertible to string, but is type %s", v.Type())
	}

	validationErrors := validation.IsQualifiedName(str)
	if len(validationErrors) != 0 {
		return fmt.Errorf("Invalid label %q: %s", str, strings.Join(validationErrors, ", "))
	}

	// Tilt assumes prefixed labels are not added by the user and thus doesn't use them
	// for resource grouping. For now, disallow users from adding prefixes so that they're
	// not confused when they don't show up in resource groups.
	if strings.Contains(str, "/") {
		return fmt.Errorf("Invalid label %q: cannot contain /", str)
	}

	*lv = LabelValue(str)

	return nil
}

type LabelOrLabelList struct {
	Values []LabelValue
}

var _ starlark.Unpacker = &LabelOrLabelList{}

// Unpack an argument that can either be expressed as
// a string or as a list of strings.
func (ls *LabelOrLabelList) Unpack(v starlark.Value) error {
	ls.Values = nil
	if v == nil {
		return nil
	}

	_, ok := v.(starlark.String)
	if ok {
		var l LabelValue
		err := l.Unpack(v)
		if err != nil {
			return err
		}
		ls.Values = []LabelValue{l}
		return nil
	}

	var iter starlark.Iterator
	switch x := v.(type) {
	case *starlark.List:
		iter = x.Iterate()
	case starlark.Tuple:
		iter = x.Iterate()
	default:
		return fmt.Errorf("value should be a label or List or Tuple of labels, but is of type %s", v.Type())
	}

	defer iter.Done()
	var item starlark.Value
	for iter.Next(&item) {
		var l LabelValue
		err := l.Unpack(item)
		if err != nil {
			return err
		}
		ls.Values = append(ls.Values, l)
	}

	return nil
}

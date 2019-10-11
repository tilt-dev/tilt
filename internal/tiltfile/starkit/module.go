package starkit

import (
	"fmt"

	"go.starlark.net/starlark"
)

// A frozen starlark module object. Can only be changed from Go.
type Module struct {
	name  string
	attrs starlark.StringDict
}

func (m Module) Freeze() {}

func (m Module) Type() string { return "module" }

func (m Module) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: module")
}

func (m Module) Attr(name string) (starlark.Value, error) {
	val := m.attrs[name]
	return val, nil
}

func (m Module) AttrNames() []string {
	keys := make([]string, 0, len(m.attrs))
	for key := range m.attrs {
		keys = append(keys, key)
	}
	return keys
}

func (m Module) Truth() starlark.Bool {
	return true
}

func (m Module) String() string {
	return fmt.Sprintf("[module: %s]", m.name)
}

var _ starlark.HasAttrs = Module{}

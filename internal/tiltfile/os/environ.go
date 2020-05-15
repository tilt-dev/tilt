package os

import (
	"fmt"
	"os"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	"github.com/tilt-dev/tilt/internal/tiltfile/value"
)

// Exposes os.Environ as a Starlark dictionary.
type Environ struct {
}

func (Environ) Clear() error {
	os.Clearenv()
	return nil
}

func (Environ) Delete(k starlark.Value) (v starlark.Value, found bool, err error) {
	str, ok := value.AsString(k)
	if !ok {
		return starlark.None, false, nil
	}

	val, found := os.LookupEnv(string(str))
	if !found {
		return starlark.None, false, nil
	}

	os.Unsetenv(string(str))

	return starlark.String(val), true, nil
}

func (Environ) Get(k starlark.Value) (v starlark.Value, found bool, err error) {
	str, ok := value.AsString(k)
	if !ok {
		return starlark.None, false, nil
	}

	val, found := os.LookupEnv(string(str))
	return starlark.String(val), found, nil
}

func environAsDict() *starlark.Dict {
	env := os.Environ()
	result := starlark.NewDict(len(env))
	for _, e := range env {
		pair := strings.SplitN(e, "=", 2)
		_ = result.SetKey(starlark.String(pair[0]), starlark.String(pair[1]))
	}
	return result
}

func (Environ) Items() []starlark.Tuple    { return environAsDict().Items() }
func (Environ) Keys() []starlark.Value     { return environAsDict().Keys() }
func (Environ) Len() int                   { return len(os.Environ()) }
func (Environ) Iterate() starlark.Iterator { return environAsDict().Iterate() }

func (Environ) SetKey(k, v starlark.Value) error {
	kStr, ok := value.AsString(k)
	if !ok {
		return fmt.Errorf("putenv() key must be a string, not %s", k.Type())
	}
	vStr, ok := value.AsString(v)
	if !ok {
		return fmt.Errorf("putenv() value must be a string, not %s", v.Type())
	}

	os.Setenv(string(kStr), string(vStr))
	return nil
}

func (Environ) String() string        { return environAsDict().String() }
func (Environ) Type() string          { return "environ" }
func (Environ) Freeze()               {}
func (Environ) Truth() starlark.Bool  { return len(os.Environ()) > 0 }
func (Environ) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: environ") }

func (e Environ) Attr(name string) (starlark.Value, error) {
	return builtinAttr(e, name, environMethods)
}
func (Environ) AttrNames() []string { return builtinAttrNames(environMethods) }
func (Environ) CompareSameType(op syntax.Token, y_ starlark.Value, depth int) (bool, error) {
	return environAsDict().CompareSameType(op, y_, depth)
}

var _ starlark.HasSetKey = Environ{}
var _ starlark.IterableMapping = Environ{}
var _ starlark.Sequence = Environ{}
var _ starlark.Comparable = Environ{}

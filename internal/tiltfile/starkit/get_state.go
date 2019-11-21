package starkit

import (
	"fmt"
	"reflect"

	"go.starlark.net/starlark"
)

func GetState(t *starlark.Thread, v interface{}) (interface{}, error) {
	typ := reflect.TypeOf(v)
	model, ok := t.Local(modelKey).(Model)
	if !ok {
		return nil, fmt.Errorf("Internal error: Starlark not initialized correctly: starkit.Model not found")
	}
	existing, ok := model.state[typ]
	if !ok {
		return nil, fmt.Errorf("internal error: no state found for %T", v)
	}

	return existing, nil
}

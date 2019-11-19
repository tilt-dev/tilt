package starkit

import (
	"fmt"
	"reflect"

	"go.starlark.net/starlark"
)

// SetState works like SetState in React. It can take a value or a function.
//
// That function should transform old state into new state. It can have the signature `func(T) T` or `func(T) (T, error)`.
//
// For example, an extension that accumulated strings might use
// SetState() like this:
//
// err := starkit.SetState(t, func(strings []string) {
//   return append([]string{newString}, strings...)
// })
//
// This would be so much easier with generics :grimace:
//
// SetState will return an error if it can't match the type
// of anything in the state store.
func SetState(t *starlark.Thread, valOrFn interface{}) error {
	typ := reflect.TypeOf(valOrFn)
	model, ok := t.Local(modelKey).(Model)
	if !ok {
		return fmt.Errorf("Internal error: Starlark not initialized correctly: starkit.Model not found")
	}

	if typ.Kind() != reflect.Func {
		return setStateVal(model, typ, valOrFn)
	} else {
		return setStateFn(model, typ, valOrFn)
	}
}

func setStateVal(model Model, typ reflect.Type, val interface{}) error {
	// If there's already a value with this type in the state store, overwrite it.
	_, ok := model.state[typ]
	if !ok {
		return fmt.Errorf("Internal error: Type not found in state store: %T", val)
	}
	model.state[typ] = val
	return nil
}

func setStateFn(model Model, typ reflect.Type, fn interface{}) error {
	// Validate the function signature.
	if typ.NumIn() != 1 ||
		typ.NumOut() < 1 || typ.NumOut() > 2 ||
		typ.In(0) != typ.Out(0) ||
		(typ.NumOut() == 2 && typ.Out(1) != reflect.TypeOf((*error)(nil)).Elem()) {
		return fmt.Errorf("Internal error: invalid SetState call: signature must be `func(T) T` or `func(t) (T, error)`")
	}

	inTyp := typ.In(0)
	// Overwrite the value in the state store.
	existing, ok := model.state[inTyp]
	if !ok {
		return fmt.Errorf("Internal error: Type not found in state store: %T", inTyp)
	}

	outs := reflect.ValueOf(fn).Call([]reflect.Value{reflect.ValueOf(existing)})

	if typ.NumOut() == 2 && !outs[1].IsNil() {
		return outs[1].Interface().(error)
	}

	// We know this is valid because of the type validation check above.
	model.state[inTyp] = outs[0].Interface()

	return nil
}

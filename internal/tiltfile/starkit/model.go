package starkit

import (
	"context"
	"fmt"
	"reflect"

	"go.starlark.net/starlark"
)

type Model struct {
	state map[reflect.Type]interface{}
}

func NewModel() Model {
	return Model{
		state: make(map[reflect.Type]interface{}),
	}
}

func (m Model) createInitState(ext StatefulExtension) error {
	v := ext.NewState()
	t := reflect.TypeOf(v)
	_, exists := m.state[t]
	if exists {
		return fmt.Errorf("Initializing extension %T: model type conflict: %T", ext, v)
	}
	m.state[t] = v
	return nil
}

func (m Model) Load(ptr interface{}) error {
	ptrVal := reflect.ValueOf(ptr)
	if ptrVal.Kind() != reflect.Ptr {
		return fmt.Errorf("Cannot load %T", ptr)
	}
	val := ptrVal.Elem()
	typ := val.Type()
	data, ok := m.state[typ]
	if !ok {
		return fmt.Errorf("Cannot load %T", ptr)
	}

	val.Set(reflect.ValueOf(data))
	return nil
}

func ModelFromThread(t *starlark.Thread) (Model, error) {
	model, ok := t.Local(modelKey).(Model)
	if !ok {
		return Model{}, fmt.Errorf("Internal error: Starlark not initialized correctly: starkit.Model not found")
	}
	return model, nil
}

func ContextFromThread(t *starlark.Thread) (context.Context, error) {
	ctx, ok := t.Local(ctxKey).(context.Context)
	if !ok {
		return nil, fmt.Errorf("Internal error Starlark not initialized correctly: starkit.Ctx not found")
	}

	return ctx, nil
}

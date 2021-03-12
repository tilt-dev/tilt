package jsonpath

import "reflect"

// A wrapper around reflect.Value that allows callers
// to call Set() on a Value retrieved from inside a map index.
//
// In the normal reflect package, the value in a map index
// is not addressable, because the internal map implementation
// might have to rearrange the map storage.
//
// In this implementation, we keep an extra reference to the
// map that "owns" the value, so that we can call Set() on it.
type Value struct {
	reflect.Value
	parentMap    reflect.Value
	parentMapKey reflect.Value
}

func Wrap(v reflect.Value) Value {
	return Value{Value: v}
}

func ValueOf(v interface{}) Value {
	return Value{Value: reflect.ValueOf(v)}
}

func (v Value) CanSet() bool {
	if v.parentMap != (reflect.Value{}) {
		return true
	}
	return v.Value.CanSet()
}

func (v *Value) Set(newV reflect.Value) {
	if v.parentMap != (reflect.Value{}) {
		v.parentMap.SetMapIndex(v.parentMapKey, newV)
		v.Value = newV
		return
	}
	v.Value.Set(newV)
}

func (v *Value) Sibling(name string) (val Value, ok bool) {
	if !v.parentMap.IsValid() {
		return Value{}, false
	}

	key := reflect.ValueOf(name)
	sib := v.parentMap.MapIndex(key)
	if !sib.IsValid() {
		return Value{}, false
	}

	return Value{
		Value:        sib,
		parentMap:    v.parentMap,
		parentMapKey: key,
	}, true
}

package view

import (
	"encoding/json"
	"reflect"
	"testing"
	"unicode"
	"unicode/utf8"
)

// The view data model is not allowed to have any private properties,
// because these properties need to be serialized to JSON for the web UI.
func TestMarshalView(t *testing.T) {
	assertCanMarshal(t, reflect.TypeOf(View{}), reflect.TypeOf(View{}))
}

// v: the type to check.
// owner: the owner of this field, for display purposes.
func assertCanMarshal(t *testing.T, v reflect.Type, owner reflect.Type) {
	// If this type does its own marshaling
	var marshal *json.Marshaler
	if v.Implements(reflect.TypeOf(marshal).Elem()) {
		return
	}

	kind := v.Kind()
	switch kind {
	case reflect.Array, reflect.Slice, reflect.Ptr:
		assertCanMarshal(t, v.Elem(), owner)
	case reflect.Map:
		assertCanMarshal(t, v.Elem(), owner)
		assertCanMarshal(t, v.Key(), owner)
	case reflect.Interface:
		// We only allow certain interfaces with a well-defined set of values.
		// NOTE(nick): I honestly think we should forbid interfaces in any data model that
		// we need to send across a network, but that's a bigger change.
		switch v.String() {
		case "error":
			// ok
			return
		case "view.ResourceInfoView":
			assertCanMarshal(t, reflect.TypeOf(K8SResourceInfo{}), v)
			assertCanMarshal(t, reflect.TypeOf(DCResourceInfo{}), v)
			assertCanMarshal(t, reflect.TypeOf(YAMLResourceInfo{}), v)
			return
		}
		t.Errorf("View needs to be serializable. This type in the view don't make sense: %s in %s", v, owner)

	case reflect.Chan, reflect.Func:
		t.Errorf("View needs to be serializable. This type in the view don't make sense: %s in %s", v, owner)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			if !isExported(field.Name) {
				t.Errorf("All fields in the View need to be serializable to web. Unexported fields are forbidden: %s in %s",
					field.Name, v)
			}
			assertCanMarshal(t, field.Type, v)
		}
	}
}

func isExported(id string) bool {
	r, _ := utf8.DecodeRuneInString(id)
	return unicode.IsUpper(r)
}

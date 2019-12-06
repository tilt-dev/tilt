package webview

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

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
		t.Errorf("View needs to be serializable. This type in the view don't make sense: %s in %s", v, owner)

	case reflect.Chan, reflect.Func:
		t.Errorf("View needs to be serializable. This type in the view don't make sense: %s in %s", v, owner)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			if !isExported(field.Name) {
				t.Errorf("All fields in the WebView need to be serializable to web. Unexported fields are forbidden: %s in %s",
					field.Name, v)
			}
			tag := field.Tag.Get("json")
			jsonName := strings.SplitN(tag, ",", 2)[0]
			if !isValidJSONField(jsonName) {
				t.Errorf("All fields in the WebView need to be serializable to valid lower-case JSON fields. Field name: %s. Json tag: %s",
					field.Name, jsonName)
			}

			assertCanMarshal(t, field.Type, v)
		}
	}
}

func isExported(id string) bool {
	r, _ := utf8.DecodeRuneInString(id)
	return unicode.IsUpper(r)
}

func isValidJSONField(field string) bool {
	if strings.Contains(field, "_") {
		return false
	}

	r, _ := utf8.DecodeRuneInString(field)
	return unicode.IsLower(r)
}

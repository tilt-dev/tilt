package webview

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

// The view data model is not allowed to have any private properties,
// because these properties need to be serialized to JSON for the web UI.
func TestMarshalView(t *testing.T) {
	assertCanMarshal(t, reflect.TypeOf(View{}), reflect.TypeOf(View{}))
}

func TestJSONRoundTrip(t *testing.T) {
	// this file can be generated via
	// curl localhost:10350/api/snapshot/aaaa | jq '.View' > view.json
	testdataPath := filepath.Join("testdata", "view.json")
	contents, err := ioutil.ReadFile(testdataPath)
	assert.NoError(t, err)

	// why is this 1.5 round trips instead of 1?
	// go produces output where && is changed to \u0026. I couldn't find a convenient mechanism to generate
	// view.json that matched this.

	// deserialize into a map[string]interface so that it'll have everything and observed will, also
	decoder := json.NewDecoder(bytes.NewBuffer(contents))
	decoder.DisallowUnknownFields()
	expected := make(map[string]interface{})
	err = decoder.Decode(&expected)
	require.NoError(t, err)

	// round-trip through an instance of `View`
	decoder = json.NewDecoder(bytes.NewBuffer(contents))
	decoder.DisallowUnknownFields()
	var view View
	err = decoder.Decode(&view)
	require.NoError(t, err)
	b := bytes.NewBuffer(nil)
	encoder := json.NewEncoder(b)
	err = encoder.Encode(view)
	require.NoError(t, err)

	// now put it back into a `map[string]interface` so that we can compare with `expected`
	decoder = json.NewDecoder(b)
	observed := make(map[string]interface{})
	err = decoder.Decode(&observed)
	require.NoError(t, err)

	require.Equal(t, expected, observed)
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

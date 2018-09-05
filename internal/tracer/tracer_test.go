package tracer

import (
	"reflect"
	"testing"
)

func TestTagStrToMap(t *testing.T) {
	s := "key1=val1,key2=val2"
	expected := map[string]string{
		"key1": "val1",
		"key2": "val2",
	}

	res := TagStrToMap(s)
	if !reflect.DeepEqual(expected, res) {
		t.Errorf("Expected result %v, got: %v", expected, res)
	}
}

func TestTagStrToMapIgnoresMalformedEntries(t *testing.T) {
	s := "key1=val1,malformed,key2=val2"
	expected := map[string]string{
		"key1": "val1",
		"key2": "val2",
	}

	res := TagStrToMap(s)
	if !reflect.DeepEqual(expected, res) {
		t.Errorf("Expected result %v, got: %v", expected, res)
	}
}

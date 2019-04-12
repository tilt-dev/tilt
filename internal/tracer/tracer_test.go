package tracer

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
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

var tracerBackendTests = []struct {
	in    string
	out   TracerBackend
	error bool
}{
	{"windmill", Windmill, false},
	{"lightstep", Lightstep, false},
	{"foo", Windmill, true},
	{"jaeger", Jaeger, false},
}

func TestStringToTracerBackend(t *testing.T) {
	for _, tt := range tracerBackendTests {
		t.Run(tt.in, func(t *testing.T) {
			actual, err := StringToTracerBackend(tt.in)
			if tt.error {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.out, actual)
		})
	}
}

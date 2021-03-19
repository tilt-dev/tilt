package visitor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// Given a set of YAML inputs, decode them into real API objects.
//
// The scheme is used to lookup the objects by group/version/kind.
func DecodeAll(scheme *runtime.Scheme, vs []Interface) ([]runtime.Object, error) {
	result := []runtime.Object{}
	for _, v := range vs {
		objs, err := Decode(scheme, v)
		if err != nil {
			return nil, err
		}
		result = append(result, objs...)
	}
	return result, nil
}

func Decode(scheme *runtime.Scheme, v Interface) ([]runtime.Object, error) {
	r, err := v.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()

	result, err := ParseStream(scheme, r)
	if err != nil {
		return nil, errors.Wrapf(err, "visiting %s", v.Name())
	}
	return result, nil
}

// Parses a stream of Tilt configuration objects.
//
// In kubectl, the CLI has to get the type information from the server in order
// to perform validation. In Tilt (today), we don't have to worry about version skew,
// so we can more aggressively validate up-front for misspelled fields
// and malformed YAML. So this parser is a bit stricter than the normal kubectl code.
func ParseStream(scheme *runtime.Scheme, r io.Reader) ([]runtime.Object, error) {
	decoder := yaml.NewYAMLOrJSONDecoder(r, 4096)
	result := []runtime.Object{}
	for {
		msg := json.RawMessage{} // First convert into json bytes
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		// Then decode into the type.
		tm := metav1.TypeMeta{}
		err := json.Unmarshal([]byte(msg), &tm)
		if err != nil {
			return nil, err
		}

		// Turn the type name into a native go object.
		obj, err := scheme.New(tm.GroupVersionKind())
		if err != nil {
			return nil, err
		}

		// Then decode the object into its native go object
		objDecoder := json.NewDecoder(bytes.NewBuffer([]byte(msg)))
		objDecoder.DisallowUnknownFields()
		if err := objDecoder.Decode(&obj); err != nil {
			return nil, fmt.Errorf("decoding %s: %v\nOriginal object:\n%s", tm, err, string([]byte(msg)))
		}

		result = append(result, obj)
	}
	return result, nil
}

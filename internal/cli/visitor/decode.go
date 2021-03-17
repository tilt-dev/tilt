package visitor

import (
	"bufio"
	"bytes"
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

func ParseStream(scheme *runtime.Scheme, r io.Reader) ([]runtime.Object, error) {
	var current bytes.Buffer
	reader := io.TeeReader(bufio.NewReader(r), &current)

	objDecoder := yaml.NewYAMLOrJSONDecoder(&current, 4096)
	typeDecoder := yaml.NewYAMLOrJSONDecoder(reader, 4096)
	result := []runtime.Object{}
	for {
		tm := metav1.TypeMeta{}
		if err := typeDecoder.Decode(&tm); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		obj, err := scheme.New(tm.GroupVersionKind())
		if err != nil {
			return nil, err
		}

		if err := objDecoder.Decode(obj); err != nil {
			if err == io.EOF {
				break
			}
			return nil, errors.Wrapf(err, "decoding %s", tm)
		}

		result = append(result, obj)
	}
	return result, nil
}

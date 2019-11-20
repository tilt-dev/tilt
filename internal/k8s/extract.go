package k8s

import (
	"fmt"
	"reflect"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ExtractPods(obj interface{}) ([]*v1.PodSpec, error) {
	extracted, err := newExtractor(reflect.TypeOf(v1.PodSpec{})).extractPointersFrom(obj)
	if err != nil {
		return nil, err
	}

	result := make([]*v1.PodSpec, len(extracted))
	for i, e := range extracted {
		c, ok := e.(*v1.PodSpec)
		if !ok {
			return nil, fmt.Errorf("extractPods: expected Pod, actual %T", e)
		}
		result[i] = c
	}
	return result, nil
}

func ExtractPodTemplateSpec(obj interface{}) ([]*v1.PodTemplateSpec, error) {
	extracted, err := newExtractor(reflect.TypeOf(v1.PodTemplateSpec{})).extractPointersFrom(obj)
	if err != nil {
		return nil, err
	}

	result := make([]*v1.PodTemplateSpec, len(extracted))
	for i, e := range extracted {
		c, ok := e.(*v1.PodTemplateSpec)
		if !ok {
			return nil, fmt.Errorf("extractPods: expected Pod, actual %T", e)
		}
		result[i] = c
	}
	return result, nil
}

func extractObjectMetas(obj interface{}, filter func(v reflect.Value) bool) ([]*metav1.ObjectMeta, error) {
	extracted, err := newExtractor(reflect.TypeOf(metav1.ObjectMeta{})).
		withFilter(filter).
		extractPointersFrom(obj)
	if err != nil {
		return nil, err
	}

	result := make([]*metav1.ObjectMeta, len(extracted))
	for i, e := range extracted {
		c, ok := e.(*metav1.ObjectMeta)
		if !ok {
			return nil, fmt.Errorf("ExtractObjectMetas: expected ObjectMeta, actual %T", e)
		}
		result[i] = c
	}
	return result, nil
}

func extractSelectors(obj interface{}, filter func(v reflect.Value) bool) ([]*metav1.LabelSelector, error) {
	extracted, err := newExtractor(reflect.TypeOf(metav1.LabelSelector{})).
		withFilter(filter).
		extractPointersFrom(obj)
	if err != nil {
		return nil, err
	}

	result := make([]*metav1.LabelSelector, len(extracted))
	for i, e := range extracted {
		c, ok := e.(*metav1.LabelSelector)
		if !ok {
			return nil, fmt.Errorf("ExtractSelectors: expected LabelSelector, actual %T", e)
		}
		result[i] = c
	}
	return result, nil
}

func extractServiceSpecs(obj interface{}) ([]*v1.ServiceSpec, error) {
	extracted, err := newExtractor(reflect.TypeOf(v1.ServiceSpec{})).
		extractPointersFrom(obj)
	if err != nil {
		return nil, err
	}

	result := make([]*v1.ServiceSpec, len(extracted))
	for i, e := range extracted {
		c, ok := e.(*v1.ServiceSpec)
		if !ok {
			return nil, fmt.Errorf("ExtractSelectors: expected ServiceSpec, actual %T", e)
		}
		result[i] = c
	}
	return result, nil
}

func extractEnvVars(obj interface{}) ([]*v1.EnvVar, error) {
	extracted, err := newExtractor(reflect.TypeOf(v1.EnvVar{})).extractPointersFrom(obj)
	if err != nil {
		return nil, err
	}

	result := make([]*v1.EnvVar, len(extracted))
	for i, e := range extracted {
		ev, ok := e.(*v1.EnvVar)
		if !ok {
			return nil, fmt.Errorf("extractEnvVars: expected %T, actual %T", v1.EnvVar{}, e)
		}
		result[i] = ev
	}
	return result, nil
}

func extractContainers(obj interface{}) ([]*v1.Container, error) {
	extracted, err := newExtractor(reflect.TypeOf(v1.Container{})).extractPointersFrom(obj)
	if err != nil {
		return nil, err
	}

	result := make([]*v1.Container, len(extracted))
	for i, e := range extracted {
		c, ok := e.(*v1.Container)
		if !ok {
			return nil, fmt.Errorf("extractContainers: expected Container, actual %T", e)
		}
		result[i] = c
	}
	return result, nil
}

type extractor struct {
	// The type we want to return pointers to
	pType reflect.Type

	// Return true to visit the value, or false to skip it.
	filter func(v reflect.Value) bool
}

func newExtractor(pType reflect.Type) extractor {
	return extractor{
		pType:  pType,
		filter: NoFilter,
	}
}

func (e extractor) withFilter(f func(v reflect.Value) bool) extractor {
	e.filter = f
	return e
}

// Get pointers to all the pType structs in this object.
func (e extractor) extractPointersFrom(obj interface{}) ([]interface{}, error) {
	v := reflect.ValueOf(obj)
	result := make([]interface{}, 0)

	// Recursively iterate over the struct fields.
	var extract func(v reflect.Value) error
	extract = func(v reflect.Value) error {
		if !e.filter(v) {
			return nil
		}

		switch v.Kind() {
		case reflect.Ptr, reflect.Interface:
			if v.IsNil() {
				return nil
			}
			return extract(v.Elem())

		case reflect.Struct:
			if v.Type() == e.pType {
				if !v.CanAddr() {
					return fmt.Errorf("Error addressing: %v", v)
				}
				result = append(result, v.Addr().Interface())
				return nil
			}

			for i := 0; i < v.NumField(); i++ {
				field := v.Field(i)
				err := extract(field)
				if err != nil {
					return err
				}
			}
			return nil

		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				field := v.Index(i)
				err := extract(field)
				if err != nil {
					return err
				}
			}
			return nil

		}
		return nil
	}

	err := extract(v)
	if err != nil {
		return nil, err
	}
	return result, nil
}

var NoFilter = func(v reflect.Value) bool {
	return true
}

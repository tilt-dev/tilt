package k8s

import (
	"fmt"
	"reflect"

	"k8s.io/api/core/v1"
)

func ExtractPods(obj interface{}) ([]*v1.PodSpec, error) {
	extracted, err := extractPointersOf(obj, reflect.TypeOf(v1.PodSpec{}))
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

func extractContainers(obj interface{}) ([]*v1.Container, error) {
	extracted, err := extractPointersOf(obj, reflect.TypeOf(v1.Container{}))
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

// Get pointers to all the container specs in this object.
func extractPointersOf(obj interface{}, pType reflect.Type) ([]interface{}, error) {
	v := reflect.ValueOf(obj)
	result := make([]interface{}, 0)

	// Recursively iterate over the struct fields.
	var extract func(v reflect.Value) error
	extract = func(v reflect.Value) error {
		switch v.Kind() {
		case reflect.Ptr, reflect.Interface:
			if v.IsNil() {
				return nil
			}
			return extract(v.Elem())

		case reflect.Struct:
			if v.Type() == pType {
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

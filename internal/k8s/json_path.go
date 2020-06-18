package k8s

import (
	"fmt"
	"reflect"

	"k8s.io/client-go/util/jsonpath"
)

// A wrapper around JSONPath with utility functions for
// locating particular types we need (like strings).
//
// Improves the error message to include the problematic path.
type JSONPath struct {
	jp   *jsonpath.JSONPath
	path string
}

func NewJSONPath(s string) (JSONPath, error) {
	jp := jsonpath.New("jp")
	err := jp.Parse(s)
	if err != nil {
		return JSONPath{}, err
	}

	return JSONPath{jp, s}, nil
}

// Extract all the strings from the given object.
// Returns an error if the object at the specified path isn't a string.
func (jp JSONPath) FindStrings(obj interface{}) ([]string, error) {
	result := []string{}
	err := jp.VisitStrings(obj, func(match reflect.Value) error {
		result = append(result, match.String())
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Visit all the strings from the given object.
// Returns an error if the object at the specified path isn't a string.
func (jp JSONPath) VisitStrings(obj interface{}, visit func(val reflect.Value) error) error {
	matches, err := jp.jp.FindResults(obj)
	if err != nil {
		return fmt.Errorf("Matching strings (json_path=%q): %v", jp.path, err)
	}

	for _, matchSet := range matches {
		for _, match := range matchSet {
			if match.Kind() != reflect.String {
				return fmt.Errorf("May only match strings (json_path=%q)\nGot Type: %s.\nGot Value: %s",
					jp.path, match.Type(), match)
			}

			err := visit(match)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (jp JSONPath) String() string {
	return jp.path
}

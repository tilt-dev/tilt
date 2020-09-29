package k8s

import (
	"fmt"

	"github.com/tilt-dev/tilt/internal/k8s/jsonpath"
)

// A wrapper around JSONPath with utility functions for
// locating particular types we need (like strings).
//
// Improves the error message to include the problematic path.
type JSONPath struct {
	path string
}

func NewJSONPath(path string) (JSONPath, error) {
	// Make sure the JSON path parses.
	jp := jsonpath.New("jp")
	err := jp.Parse(path)
	if err != nil {
		return JSONPath{}, err
	}

	return JSONPath{
		path: path,
	}, nil
}

// Extract all the strings from the given object.
// Returns an error if the object at the specified path isn't a string.
func (jp JSONPath) FindStrings(obj interface{}) ([]string, error) {
	result := []string{}
	err := jp.VisitStrings(obj, func(match jsonpath.Value, s string) error {
		result = append(result, s)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Visit all the strings from the given object.
//
// Returns an error if the object at the specified path isn't a string.
func (jp JSONPath) VisitStrings(obj interface{}, visit func(val jsonpath.Value, str string) error) error {
	return jp.Visit(obj, func(match jsonpath.Value) error {
		val := match.Interface()
		str, ok := val.(string)
		if !ok {
			return fmt.Errorf("May only match strings (json_path=%q)\nGot Type: %T\nGot Value: %s",
				jp.path, val, val)
		}

		return visit(match, str)
	})
}

// Visit all the values from the given object on this path.
func (jp JSONPath) Visit(obj interface{}, visit func(val jsonpath.Value) error) error {
	// JSONPath is stateful and not thread-safe, so we need to parse a new one
	// each time
	matcher := jsonpath.New("jp")
	err := matcher.Parse(jp.path)
	if err != nil {
		return fmt.Errorf("Matching (json_path=%q): %v", jp.path, err)
	}

	matches, err := matcher.FindResults(obj)
	if err != nil {
		return fmt.Errorf("Matching (json_path=%q): %v", jp.path, err)
	}

	for _, matchSet := range matches {
		for _, match := range matchSet {
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

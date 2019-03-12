package k8s

import (
	"bytes"
	"strings"

	"k8s.io/client-go/util/jsonpath"
)

// This is just a wrapper around k8s jsonpath, mostly because k8s jsonpath doesn't produce errors containing
// the problematic path
// (and as long as we're here, deal with some of its other annoyances, like taking a "name" that doesn't do anything,
// and having a separate "Parse" step to make an instance actually useful, and making you use an io.Writer, and
// wrapping string results in quotes)
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

// Gets the value at the specified path
// NB: currently strips away surrounding quotes, which the underlying parser includes in its return value
// If, at some point, we want to distinguish between, e.g., ints and strings by the presence of quotes, this
// will need to be revisited.
func (jp JSONPath) Execute(obj interface{}) (string, error) {
	buf := &bytes.Buffer{}
	err := jp.jp.Execute(buf, obj)
	if err != nil {
		return "", err
	}
	return strings.Trim(buf.String(), "\""), nil
}

func (jp JSONPath) String() string {
	return jp.path
}

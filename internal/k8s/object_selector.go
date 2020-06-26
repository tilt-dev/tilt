package k8s

import (
	"fmt"
	"regexp"

	"github.com/pkg/errors"
)

// A selector matches an entity if all non-empty selector fields match the corresponding entity fields
type ObjectSelector struct {
	apiVersion       *regexp.Regexp
	apiVersionString string
	kind             *regexp.Regexp
	kindString       string

	// TODO(dmiller): do something like this instead https://github.com/tilt-dev/tilt/blob/c2b2df88de3777eed5f1bb9f54b5c555707c8b42/internal/container/selector.go#L9
	name            *regexp.Regexp
	nameString      string
	namespace       *regexp.Regexp
	namespaceString string
}

// TODO(dmiller): this function and newPartialMatchK8sObjectSelector
// should be written in to a form that can be used like this
// x := re{pattern: name, ignoreCase: true, fullMatch: true}
// x.compile()
// rather than passing around and mutating regex strings

// Creates a new ObjectSelector
// If an arg is an empty string it will become an empty regex that matches all input
// Otherwise the arg must match the input exactly
func NewFullmatchCaseInsensitiveObjectSelector(apiVersion string, kind string, name string, namespace string) (ObjectSelector, error) {
	ret := ObjectSelector{apiVersionString: apiVersion, kindString: kind, nameString: name, namespaceString: namespace}
	var err error

	ret.apiVersion, err = regexp.Compile(exactOrEmptyRegex(apiVersion))
	if err != nil {
		return ObjectSelector{}, errors.Wrap(err, "error parsing apiVersion regexp")
	}

	ret.kind, err = regexp.Compile(exactOrEmptyRegex(kind))
	if err != nil {
		return ObjectSelector{}, errors.Wrap(err, "error parsing kind regexp")
	}

	ret.name, err = regexp.Compile(exactOrEmptyRegex(name))
	if err != nil {
		return ObjectSelector{}, errors.Wrap(err, "error parsing name regexp")
	}

	ret.namespace, err = regexp.Compile(exactOrEmptyRegex(namespace))
	if err != nil {
		return ObjectSelector{}, errors.Wrap(err, "error parsing namespace regexp")
	}

	return ret, nil
}

func makeCaseInsensitive(s string) string {
	if s == "" {
		return s
	} else {
		return "(?i)" + s
	}
}

func exactOrEmptyRegex(s string) string {
	if s != "" {
		s = fmt.Sprintf("^%s$", makeCaseInsensitive(regexp.QuoteMeta(s)))
	}
	return s
}

// Create a selector that matches the Kind. Useful for testing.
func MustKindSelector(kind string) ObjectSelector {
	sel, err := NewFullmatchCaseInsensitiveObjectSelector("", kind, "", "")
	if err != nil {
		panic(err)
	}
	return sel
}

// Create a selector that matches the Name. Useful for testing.
func MustNameSelector(name string) ObjectSelector {
	sel, err := NewFullmatchCaseInsensitiveObjectSelector("", "", name, "")
	if err != nil {
		panic(err)
	}
	return sel
}

// Creates a new ObjectSelector
// If an arg is an empty string, it will become an empty regex that matches all input
// Otherwise the arg will match input from the beginning (prefix matching)
func NewPartialMatchObjectSelector(apiVersion string, kind string, name string, namespace string) (ObjectSelector, error) {
	ret := ObjectSelector{apiVersionString: apiVersion, kindString: kind, nameString: name, namespaceString: namespace}
	var err error

	ret.apiVersion, err = regexp.Compile(makeCaseInsensitive(apiVersion))
	if err != nil {
		return ObjectSelector{}, errors.Wrap(err, "error parsing apiVersion regexp")
	}

	ret.kind, err = regexp.Compile(makeCaseInsensitive(kind))
	if err != nil {
		return ObjectSelector{}, errors.Wrap(err, "error parsing kind regexp")
	}

	ret.name, err = regexp.Compile(makeCaseInsensitive(name))
	if err != nil {
		return ObjectSelector{}, errors.Wrap(err, "error parsing name regexp")
	}

	ret.namespace, err = regexp.Compile(makeCaseInsensitive(namespace))
	if err != nil {
		return ObjectSelector{}, errors.Wrap(err, "error parsing namespace regexp")
	}

	return ret, nil
}

func (k ObjectSelector) Matches(e K8sEntity) bool {
	gvk := e.GVK()
	return k.apiVersion.MatchString(gvk.GroupVersion().String()) &&
		k.kind.MatchString(gvk.Kind) &&
		k.name.MatchString(e.Name()) &&
		k.namespace.MatchString(e.Namespace().String())
}

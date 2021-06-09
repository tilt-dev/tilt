package k8s

import (
	"fmt"
	"regexp"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// A selector matches an entity if all non-empty selector fields match the corresponding entity fields
type ObjectSelector struct {
	spec v1alpha1.ObjectSelector

	apiVersion *regexp.Regexp
	kind       *regexp.Regexp
	name       *regexp.Regexp
	namespace  *regexp.Regexp
}

func ParseObjectSelector(spec v1alpha1.ObjectSelector) (ObjectSelector, error) {
	ret := ObjectSelector{spec: spec}
	var err error

	ret.apiVersion, err = regexp.Compile(spec.APIVersionRegexp)
	if err != nil {
		return ObjectSelector{}, errors.Wrap(err, "error parsing apiVersion regexp")
	}

	ret.kind, err = regexp.Compile(spec.KindRegexp)
	if err != nil {
		return ObjectSelector{}, errors.Wrap(err, "error parsing kind regexp")
	}

	ret.name, err = regexp.Compile(spec.NameRegexp)
	if err != nil {
		return ObjectSelector{}, errors.Wrap(err, "error parsing name regexp")
	}

	ret.namespace, err = regexp.Compile(spec.NamespaceRegexp)
	if err != nil {
		return ObjectSelector{}, errors.Wrap(err, "error parsing namespace regexp")
	}

	return ret, nil
}

var splitOptions = sliceutils.NewEscapeSplitOptions()

func SelectorStringFromParts(parts []string) string {
	return sliceutils.EscapeAndJoin(parts, splitOptions)
}

// format is <name:required>:<kind:optional>:<namespace:optional>
func SelectorFromString(s string) (ObjectSelector, error) {
	parts, err := sliceutils.UnescapeAndSplit(s, splitOptions)
	if err != nil {
		return ObjectSelector{}, err
	}
	if len(s) == 0 {
		return ObjectSelector{}, fmt.Errorf("selector can't be empty")
	}
	if len(parts) == 1 {
		return NewFullmatchCaseInsensitiveObjectSelector("", "", parts[0], "")
	}
	if len(parts) == 2 {
		return NewFullmatchCaseInsensitiveObjectSelector("", parts[1], parts[0], "")
	}
	if len(parts) == 3 {
		return NewFullmatchCaseInsensitiveObjectSelector("", parts[1], parts[0], parts[2])
	}

	return ObjectSelector{}, fmt.Errorf("Too many parts in selector. Selectors must contain between 1 and 3 parts (colon separated), found %d parts in %s", len(parts), s)
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
	return ParseObjectSelector(v1alpha1.ObjectSelector{
		APIVersionRegexp: exactOrEmptyRegex(apiVersion),
		KindRegexp:       exactOrEmptyRegex(kind),
		NameRegexp:       exactOrEmptyRegex(name),
		NamespaceRegexp:  exactOrEmptyRegex(namespace),
	})
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
	return ParseObjectSelector(v1alpha1.ObjectSelector{
		APIVersionRegexp: makeCaseInsensitive(apiVersion),
		KindRegexp:       makeCaseInsensitive(kind),
		NameRegexp:       makeCaseInsensitive(name),
		NamespaceRegexp:  makeCaseInsensitive(namespace),
	})
}

func (o1 ObjectSelector) EqualsSelector(o2 ObjectSelector) bool {
	return cmp.Equal(o1.spec, o2.spec)
}

func (k ObjectSelector) Matches(e K8sEntity) bool {
	gvk := e.GVK()
	return k.apiVersion.MatchString(gvk.GroupVersion().String()) &&
		k.kind.MatchString(gvk.Kind) &&
		k.name.MatchString(e.Name()) &&
		k.namespace.MatchString(e.Namespace().String())
}

func (k ObjectSelector) ToSpec() v1alpha1.ObjectSelector {
	return k.spec
}

package container

import (
	"fmt"

	"github.com/docker/distribution/reference"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type MatchType int

const (
	matchName MatchType = iota
	matchExact
)

type RefSelector struct {
	ref       reference.Named
	matchType MatchType
}

func SelectorFromImageMap(spec v1alpha1.ImageMapSpec) (RefSelector, error) {
	ref, err := reference.ParseNamed(spec.Selector)
	if err != nil {
		return RefSelector{}, fmt.Errorf("parsing image map spec (%s): %v", spec.Selector, err)
	}
	matchType := matchName
	if spec.MatchExact {
		matchType = matchExact
	}
	return RefSelector{
		ref:       ref,
		matchType: matchType,
	}, nil
}

func NameSelector(ref reference.Named) RefSelector {
	return NewRefSelector(reference.TrimNamed(ref))
}

func NewRefSelector(ref reference.Named) RefSelector {
	matchType := matchName
	_, hasTag := ref.(reference.NamedTagged)
	if hasTag {
		matchType = matchExact
	}
	return RefSelector{
		ref:       ref,
		matchType: matchType,
	}
}

func MustParseSelector(s string) RefSelector {
	return NewRefSelector(MustParseNamed(s))
}

func MustParseTaggedSelector(s string) RefSelector {
	return NewRefSelector(MustParseNamedTagged(s))
}

func (s RefSelector) RefsEqual(other RefSelector) bool {
	return s.ref.String() == other.ref.String()
}

func (s RefSelector) MatchExact() bool {
	return s.matchType == matchExact
}

func (s RefSelector) WithNameMatch() RefSelector {
	s.matchType = matchName
	return s
}

func (s RefSelector) WithExactMatch() RefSelector {
	s.matchType = matchExact
	return s
}

func (s RefSelector) MatchesAny(toMatch []reference.Named) bool {
	for _, ref := range toMatch {
		if s.Matches(ref) {
			return true
		}
	}
	return false
}

func (s RefSelector) Matches(toMatch reference.Named) bool {
	if s.ref == nil {
		return false
	}

	if s.matchType == matchName {
		return toMatch.Name() == s.ref.Name()
	}
	return toMatch.String() == s.ref.String()
}

func (s RefSelector) Empty() bool {
	return s.ref == nil
}

func (s RefSelector) RefName() string {
	return s.ref.Name()
}

func (s RefSelector) RefFamiliarName() string {
	return reference.FamiliarName(s.ref)
}

func (s RefSelector) RefFamiliarString() string {
	return reference.FamiliarString(s.ref)
}

// AsNamedOnly returns the Ref as a Named, REMOVING THE TAG if one exists
func (s RefSelector) AsNamedOnly() reference.Named {
	return reference.TrimNamed(s.ref)
}

func (s RefSelector) String() string {
	if s.ref == nil {
		return ""
	}
	return s.ref.String()
}

func AnyMatch(toMatch []reference.Named, selectors []RefSelector) bool {
	for _, ref := range toMatch {
		for _, sel := range selectors {
			if sel.Matches(ref) {
				return true
			}
		}
	}
	return false
}

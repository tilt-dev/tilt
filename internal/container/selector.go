package container

import (
	"github.com/docker/distribution/reference"
)

type MatchType int

const (
	matchName MatchType = iota
	matchExact
)

type RefSelector struct {
	// the ref to use when trying to match against a ref (i.e., pre-default-registry)
	matchRef reference.Named
	// the ref to use when injecting this ref (i.e., post-default-registry)
	injectRef reference.Named
	matchType MatchType
}

func NameSelector(ref reference.Named) RefSelector {
	r := reference.TrimNamed(ref)
	return newRefSelector(r, r)
}

func NewRefSelector(ref reference.Named) RefSelector {
	return newRefSelector(ref, ref)
}

func MustParseSelector(s string) RefSelector {
	r := MustParseNamed(s)
	return newRefSelector(r, r)
}

func MustParseTaggedSelector(s string) RefSelector {
	r := MustParseNamedTagged(s)
	return newRefSelector(r, r)
}

func (s RefSelector) WithDefaultRegistry(defaultRegistry string) (RefSelector, error) {
	ref, err := replaceNamed(defaultRegistry, s.matchRef)
	if err != nil {
		return RefSelector{}, err
	}
	s.injectRef = ref
	return s, nil
}

func newRefSelector(matchRef, injectRef reference.Named) RefSelector {
	matchType := matchName
	_, hasTag := matchRef.(reference.NamedTagged)
	if hasTag {
		matchType = matchExact
	}

	return RefSelector{
		matchRef:  matchRef,
		injectRef: injectRef,
		matchType: matchType,
	}
}

func (s RefSelector) RefsEqual(other RefSelector) bool {
	return s.matchRef.String() == other.matchRef.String()
}

func (s RefSelector) WithExactMatch() RefSelector {
	s.matchType = matchExact
	return s
}

func (s RefSelector) Matches(toMatch reference.Named) bool {
	if s.matchRef == nil {
		return false
	}

	if s.matchType == matchName {
		return toMatch.Name() == s.matchRef.Name()
	}
	return toMatch.String() == s.matchRef.String()
}

func (s RefSelector) Empty() bool {
	return s.matchRef == nil
}

func (s RefSelector) MatchName() string {
	return s.matchRef.Name()
}

func (s RefSelector) InjectName() string {
	return s.injectRef.Name()
}

func (s RefSelector) AsNamedOnly() reference.Named {
	return reference.TrimNamed(s.injectRef)
}

func (s RefSelector) MatchString() string {
	if s.matchRef == nil {
		return ""
	}
	return s.matchRef.String()
}

func (s RefSelector) InjectString() string {
	if s.injectRef == nil {
		return ""
	}
	return s.injectRef.String()
}

func (s RefSelector) String() string {
	return s.MatchString()
}

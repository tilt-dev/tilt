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
	ref       reference.Named
	matchType MatchType
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

func (s RefSelector) WithExactMatch() RefSelector {
	s.matchType = matchExact
	return s
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

func (s RefSelector) AsNamedOnly() reference.Named {
	return reference.TrimNamed(s.ref)
}

func (s RefSelector) String() string {
	if s.ref == nil {
		return ""
	}
	return s.ref.String()
}

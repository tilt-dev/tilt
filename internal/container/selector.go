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
	Ref       reference.Named
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
		Ref:       ref,
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
	return s.Ref.String() == other.Ref.String()
}

func (s RefSelector) WithNameMatch() RefSelector {
	s.matchType = matchName
	return s
}

func (s RefSelector) WithExactMatch() RefSelector {
	s.matchType = matchExact
	return s
}

func (s RefSelector) Matches(toMatch reference.Named) bool {
	if s.Ref == nil {
		return false
	}

	if s.matchType == matchName {
		return toMatch.Name() == s.Ref.Name()
	}
	return toMatch.String() == s.Ref.String()
}

func (s RefSelector) Empty() bool {
	return s.Ref == nil
}

func (s RefSelector) RefName() string {
	return s.Ref.Name()
}

// AsNamedOnly returns the Ref as a Named, REMOVING THE TAG if one exists
func (s RefSelector) AsNamedOnly() reference.Named {
	return reference.TrimNamed(s.Ref)
}

func (s RefSelector) String() string {
	if s.Ref == nil {
		return ""
	}
	return s.Ref.String()
}

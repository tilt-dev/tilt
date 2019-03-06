package container

import "github.com/docker/distribution/reference"

type RefSelector struct {
	ref reference.Named
}

func NewRefSelector(ref reference.Named) RefSelector {
	return RefSelector{ref: ref}
}

func MustParseSelector(s string) RefSelector {
	return NewRefSelector(MustParseNamed(s))
}

func MustParseTaggedSelector(s string) RefSelector {
	return NewRefSelector(MustParseNamedTagged(s))
}

func (s RefSelector) Matches(toMatch reference.Named) bool {
	if s.ref == nil {
		return false
	}
	return toMatch.Name() == s.ref.Name()
}

func (s RefSelector) Empty() bool {
	return s.ref == nil
}

func (s RefSelector) Name() string {
	return s.ref.Name()
}

func (s RefSelector) AsNamedOnly() reference.Named {
	return reference.TrimNamed(s.ref)
}

// TODO(nick): This method is only provided for legacy compatibility.
// All uses of this really should not be using the full ref.
func (s RefSelector) AsRef() reference.Named {
	return s.ref
}

func (s RefSelector) String() string {
	if s.ref == nil {
		return ""
	}
	return s.ref.String()
}

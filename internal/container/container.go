package container

import (
	"fmt"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
)

type ID string
type Name string

func (id ID) Empty() bool    { return id.String() == "" }
func (id ID) String() string { return string(id) }
func (id ID) ShortStr() string {
	if len(string(id)) > 10 {
		return string(id)[:10]
	}
	return string(id)
}

func (n Name) String() string { return string(n) }

func ParseNamed(s string) (reference.Named, error) {
	return reference.ParseNormalizedNamed(s)
}

func ParseNamedMulti(strs []string) ([]reference.Named, error) {
	var err error
	res := make([]reference.Named, len(strs))
	for i, s := range strs {
		res[i], err = reference.ParseNormalizedNamed(s)
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

func ParseNamedTagged(s string) (reference.NamedTagged, error) {
	ref, err := reference.ParseNormalizedNamed(s)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing %q", s)
	}

	nt, ok := ref.(reference.NamedTagged)
	if !ok {
		return nil, fmt.Errorf("Expected reference %q to contain a tag", s)
	}
	return nt, nil
}

func MustParseNamedTagged(s string) reference.NamedTagged {
	nt, err := ParseNamedTagged(s)
	if err != nil {
		panic(err)
	}
	return nt
}

func MustParseNamed(s string) reference.Named {
	n, err := reference.ParseNormalizedNamed(s)
	if err != nil {
		panic(fmt.Sprintf("MustParseNamed(%q): %v", s, err))
	}
	return n
}

func MustWithTag(name reference.Named, tag string) reference.NamedTagged {
	nt, err := reference.WithTag(name, tag)
	if err != nil {
		panic(err)
	}
	return nt
}

// ImageNamesEqual returns true if the references correspond to the same named
// image.
//
// If either reference is not a valid named image reference, false is returned.
//
// For example: `reg.example.com/foo:abc` & `reg.example.com/foo:def` are equal
// because the named image is `reg.example.com/foo` in both cases.
func ImageNamesEqual(a, b string) bool {
	aRef, err := reference.ParseNormalizedNamed(a)
	if err != nil {
		return false
	}

	bRef, err := reference.ParseNormalizedNamed(b)
	if err != nil {
		return false
	}

	return reference.FamiliarName(aRef) == reference.FamiliarName(bRef)
}

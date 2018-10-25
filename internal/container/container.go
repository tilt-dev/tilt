package container

import (
	"fmt"

	"github.com/docker/distribution/reference"
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

func ParseNamedTagged(s string) (reference.NamedTagged, error) {
	ref, err := reference.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %v", s, err)
	}

	nt, ok := ref.(reference.NamedTagged)
	if !ok {
		return nil, fmt.Errorf("could not parse ref %s as NamedTagged", ref)
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
	n, err := reference.ParseNamed(s)
	if err != nil {
		panic(err)
	}
	return n
}

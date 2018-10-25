package container

import (
	"fmt"

	"github.com/docker/distribution/reference"
)

type ContainerID string
type ContainerName string

func (cID ContainerID) Empty() bool    { return cID.String() == "" }
func (cID ContainerID) String() string { return string(cID) }
func (cID ContainerID) ShortStr() string {
	if len(string(cID)) > 10 {
		return string(cID)[:10]
	}
	return string(cID)
}

func (n ContainerName) String() string { return string(n) }

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

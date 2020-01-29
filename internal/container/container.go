package container

import (
	"fmt"
	"strings"

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

func ShortStrs(ids []ID) string {
	shortStrs := make([]string, len(ids))
	for i, id := range ids {
		shortStrs[i] = id.ShortStr()
	}
	return strings.Join(shortStrs, ", ")
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
		return nil, errors.Wrapf(err, "parsing %s", s)
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

func NewIDSet(ids ...ID) map[ID]bool {
	result := make(map[ID]bool, len(ids))
	for _, id := range ids {
		result[id] = true
	}
	return result
}

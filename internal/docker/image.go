package docker

import (
	"github.com/docker/distribution/reference"
)

const TagLatest = "latest"

// For use storing reference.NamedTagged as a map key, since we can't rely on the
// two different underlying representations the same name+tag combo being equivalent.
type ImgNameAndTag struct {
	Name string
	Tag  string
}

func ToImgNameAndTag(nt reference.NamedTagged) ImgNameAndTag {
	return ImgNameAndTag{
		Name: nt.Name(),
		Tag:  nt.Tag(),
	}
}

func NormalizeRefName(ref string) (string, error) {
	named, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		return "", err
	}
	return named.String(), nil
}

func MustNormalizeRefName(ref string) string {
	normalized, err := NormalizeRefName(ref)
	if err != nil {
		panic(err)
	}
	return normalized
}

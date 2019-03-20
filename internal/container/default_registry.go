package container

import (
	"fmt"
	"regexp"

	"github.com/docker/distribution/reference"
)

// Produces a new image name that is in the specified registry.
// The name might be ugly, favoring uniqueness and simplicity and assuming that the prettiness of ephemeral dev image
// names is not that important.
func replaceNamedTagged(defaultRegistry string, name reference.NamedTagged) (reference.NamedTagged, error) {
	newN, err := replaceNamed(defaultRegistry, MustParseNamed(name.Name()))
	if err != nil {
		return nil, err
	}

	newNT, err := reference.WithTag(newN, name.Tag())
	if err != nil {
		return nil, err
	}

	return newNT, nil
}

var escapeRegex = regexp.MustCompile(`[/:@]`)

func escapeName(s string) string {
	return string(escapeRegex.ReplaceAll([]byte(s), []byte("_")))
}

// Produces a new image name that is in the specified registry.
// The name might be ugly, favoring uniqueness and simplicity and assuming that the prettiness of ephemeral dev image
// names is not that important.
func replaceNamed(defaultRegistry string, name reference.Named) (reference.Named, error) {
	if defaultRegistry == "" {
		return name, nil
	}

	ns := name.String()

	newNs := fmt.Sprintf("%s/%s", defaultRegistry, escapeName(ns))
	newN, err := reference.ParseNamed(newNs)
	if err != nil {
		return nil, err
	}

	return newN, nil
}

package container

import (
	"fmt"
	"regexp"

	"github.com/docker/distribution/reference"
)

var escapeRegex = regexp.MustCompile(`[/:@]`)

func escapeName(s string) string {
	return string(escapeRegex.ReplaceAll([]byte(s), []byte("_")))
}

// Produces a new image name that is in the specified registry.
// The name might be ugly, favoring uniqueness and simplicity and assuming that the prettiness of ephemeral dev image
// names is not that important.
func ReplaceRegistry(defaultRegistry string, rs RefSelector) (reference.Named, error) {
	if defaultRegistry == "" {
		return rs.AsNamedOnly(), nil
	}

	newNs := fmt.Sprintf("%s/%s", defaultRegistry, escapeName(rs.RefName()))
	newN, err := reference.ParseNamed(newNs)
	if err != nil {
		return nil, err
	}

	return newN, nil
}

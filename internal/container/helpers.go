package container

import "github.com/docker/distribution/reference"

func FamiliarString(ref reference.Reference) string {
	s, ok := ref.(RefSelector)
	if ok {
		return s.RefFamiliarString()
	}
	return reference.FamiliarString(ref)
}

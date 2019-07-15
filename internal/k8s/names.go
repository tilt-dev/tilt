package k8s

import (
	"fmt"
	"strings"
)

// Calculates names for workloads by using the shortest uniquely matching identifiers
func UniqueNames(es []K8sEntity, minComponents int) []string {
	ret := make([]string, len(es))
	// how many resources potentially map to a given name
	counts := make(map[string]int)

	// count how many entities want each potential name
	for _, e := range es {
		for _, name := range potentialNames(e, minComponents) {
			counts[name]++
		}
	}

	// for each entity, take the shortest name that is uniquely wanted by that entity
	for i, e := range es {
		names := potentialNames(e, minComponents)
		for _, name := range names {
			if counts[name] == 1 {
				ret[i] = name
				break
			}
		}

		if ret[i] == "" {
			// If we hit this case, this means we have two resources with the same
			// name/kind/namespace/group This usually means the user is trying to
			// deploy the same resource twice. Kubernetes will not treat these as
			// unique.
			//
			// We should surface a warning or error about this somewhere else that has
			// more context on how to fix it.
			// https://github.com/windmilleng/tilt/issues/1852
			//
			// But for now, append the index to the name to make it unique
			ret[i] = fmt.Sprintf("%s:%d", names[len(names)-1], i)
		}
	}

	return ret
}

// returns a list of potential names, in order of preference
func potentialNames(e K8sEntity, minComponents int) []string {
	components := []string{
		e.Name(),
		e.Kind.Kind,
		e.Namespace().String(),
		e.Kind.Group,
	}
	var ret []string
	for i := minComponents - 1; i < len(components); i++ {
		ret = append(ret, strings.ToLower(strings.Join(components[:i+1], ":")))
	}
	return ret
}

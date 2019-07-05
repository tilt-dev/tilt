package k8s

import (
	"fmt"
	"strings"
)

// Calculates names for workloads by using the shortest uniquely matching identifiers
func UniqueNames(es []K8sEntity, minComponents int) ([]string, error) {
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
			return nil, fmt.Errorf("unable to find a unique resource name for '%s'", names[len(names)-1])
		}
	}

	return ret, nil
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

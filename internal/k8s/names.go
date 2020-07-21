package k8s

import (
	"fmt"
	"strings"
)

const FmtduplicateYamlDetectedError = "Duplicate YAML Entity(s): [%s] has been detected across one or more resources.  Only one specification per entity can be applied to the cluster; to ensure expected behavior, remove the duplicate specifications."

func DuplicateYamlDetectedError(duplicatedYaml string) string {
	return fmt.Sprintf(FmtduplicateYamlDetectedError, duplicatedYaml)
}

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
			// Previously, we appended an index to the duplicate yaml entity, but as of July 21st 2020, we return an error
			//if we've detected a duplicate yaml entity.
			return nil, fmt.Errorf(DuplicateYamlDetectedError(names[len(names)-1]))
		}
	}

	return ret, nil
}

// FragmentsToEntities maps all possible fragments (e.g. foo, foo:secret, foo:secret:default) to the k8s entity or entities that they correspond to
func FragmentsToEntities(es []K8sEntity) map[string][]K8sEntity {
	ret := make(map[string][]K8sEntity, len(es))

	for _, e := range es {
		names := potentialNames(e, 1)
		for _, name := range names {
			if a, ok := ret[name]; ok {
				ret[name] = append(a, e)
			} else {
				ret[name] = []K8sEntity{e}
			}
		}
	}

	return ret
}

// returns a list of potential names, in order of preference
func potentialNames(e K8sEntity, minComponents int) []string {
	gvk := e.GVK()

	// Empty string is synonymous with the core group
	// Poorly documented, but check it out here https://kubernetes.io/docs/reference/access-authn-authz/authorization/#review-your-request-attributes
	group := gvk.Group
	if group == "" {
		group = "core"
	}

	components := []string{
		e.Name(),
		gvk.Kind,
		e.Namespace().String(),
		group,
	}
	var ret []string
	for i := minComponents - 1; i < len(components); i++ {
		ret = append(ret, strings.ToLower(strings.Join(components[:i+1], ":")))
	}
	return ret
}

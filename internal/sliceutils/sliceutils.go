package sliceutils

import "sort"

// Deduplicate and sort a slice of strings.
func DedupedAndSorted(slice []string) (result []string) {
	seen := map[string]bool{}

	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	sort.Strings(result)
	return result
}

func StringSliceEquals(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i, e1 := range a {
		e2 := b[i]
		if e1 != e2 {
			return false
		}
	}

	return true
}

// returns a slice that consists of `a`, in order, followed by elements of `b` that are not in `a`
func AppendWithoutDupes(a []string, b ...string) []string {
	seen := make(map[string]bool)
	for _, s := range a {
		seen[s] = true
	}

	ret := append([]string{}, a...)
	for _, s := range b {
		if !seen[s] {
			ret = append(ret, s)
		}
	}

	return ret
}

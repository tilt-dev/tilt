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
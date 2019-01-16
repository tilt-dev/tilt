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

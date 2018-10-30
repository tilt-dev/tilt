package sliceutils

func DedupeStringSlice(slice []string) (result []string) {
	seen := map[string]bool{}

	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

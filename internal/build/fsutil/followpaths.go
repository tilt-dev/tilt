package fsutil

import "strings"

// dedupePaths expects input as a sorted list
func dedupePaths(in []string) []string {
	out := make([]string, 0, len(in))
	var last string
	for _, s := range in {
		// if one of the paths is root there is no filter
		if s == "." {
			return nil
		}
		if strings.HasPrefix(s, last+"/") {
			continue
		}
		out = append(out, s)
		last = s
	}
	return out
}

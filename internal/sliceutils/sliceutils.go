package sliceutils

import (
	"fmt"
	"sort"
	"strings"
)

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

// Quote each string in the list and separate them by commas.
func QuotedStringList(list []string) string {
	result := make([]string, len(list))
	for i, s := range list {
		result[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(result, ", ")
}

func BulletedIndentedStringList(list []string) string {
	if len(list) == 0 {
		return ""
	}

	return "\t- " + strings.Join(list, "\n\t- ")
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

// StringSliceStartsWith returns true if slice A starts with the given elem.
func StringSliceStartsWith(a []string, elem string) bool {
	if len(a) == 0 {
		return false
	}

	return a[0] == elem
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

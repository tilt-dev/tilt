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

// return true iff `a` has the same elements as `b`, ignoring order
func StringSliceSameElements(a, b []string) bool {
	aElems := make(map[string]int)
	bElems := make(map[string]int)
	for _, s := range a {
		aElems[s] += 1
	}

	for _, s := range b {
		bElems[s] += 1
	}

	if len(aElems) != len(bElems) {
		return false
	}

	for s, c1 := range aElems {
		if c1 != bElems[s] {
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

type EscapeSplitOptions struct {
	Delimiter  rune
	EscapeChar rune
}

func NewEscapeSplitOptions() EscapeSplitOptions {
	return EscapeSplitOptions{
		Delimiter:  ':',
		EscapeChar: '\\',
	}
}

func UnescapeAndSplit(s string, opts EscapeSplitOptions) ([]string, error) {
	var parts []string
	escapeNextChar := false
	cur := ""
	for i, r := range s {
		if escapeNextChar {
			if r == opts.Delimiter || r == opts.EscapeChar {
				cur += string(r)
			} else {
				// grab the 6 chars around the invalid char to make it easier to find the offending string from the error
				snippetStart := i - 3
				if snippetStart < 0 {
					snippetStart = 0
				}
				snippetEnd := i + 3
				if snippetEnd > len(s) {
					snippetEnd = len(s)
				}

				return nil, fmt.Errorf("invalid escape sequence '%c%c' in '%s'", opts.EscapeChar, r, s[snippetStart:snippetEnd])
			}
			escapeNextChar = false
		} else {
			switch r {
			case opts.Delimiter:
				parts = append(parts, cur)
				cur = ""
			case opts.EscapeChar:
				escapeNextChar = true
			default:
				cur += string(r)
			}
		}
	}
	parts = append(parts, cur)

	return parts, nil
}

func quotePart(s string, opts EscapeSplitOptions) string {
	for _, r := range []rune{opts.EscapeChar, opts.Delimiter} {
		s = strings.ReplaceAll(s, string(r), fmt.Sprintf("%c%c", opts.EscapeChar, r))
	}
	return s
}

func EscapeAndJoin(parts []string, opts EscapeSplitOptions) string {
	ret := strings.Builder{}
	for i, part := range parts {
		if i != 0 {
			ret.WriteRune(opts.Delimiter)
		}
		_, _ = ret.WriteString(quotePart(part, opts))
	}

	return ret.String()
}

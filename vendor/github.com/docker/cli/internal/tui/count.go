// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.23

package tui

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

func cleanANSI(s string) string {
	for {
		start := strings.Index(s, "\x1b")
		if start == -1 {
			return s
		}
		end := strings.Index(s[start:], "m")
		if end == -1 {
			return s
		}
		s = s[:start] + s[start+end+1:]
	}
}

// Width returns the width of the string, ignoring ANSI escape codes.
// Not all ANSI escape codes are supported yet.
func Width(s string) int {
	return runewidth.StringWidth(cleanANSI(s))
}

// Ellipsis truncates a string to a given number of runes with an ellipsis at the end.
// It tries to persist the ANSI escape sequences.
func Ellipsis(s string, length int) string {
	out := make([]rune, 0, length)
	ln := 0
	inEscape := false
	tooLong := false

	for _, r := range s {
		if r == '\x1b' {
			out = append(out, r)
			inEscape = true
			continue
		}
		if inEscape {
			out = append(out, r)
			if r == 'm' {
				inEscape = false
				if tooLong {
					break
				}
			}
			continue
		}

		ln += 1
		if ln == length {
			tooLong = true
		}
		if !tooLong {
			out = append(out, r)
		}
	}

	if tooLong {
		return string(out) + "…"
	}
	return string(out)
}

/*
Package shellescape provides the shellescape.Quote to escape arbitrary
strings for a safe use as command line arguments in the most common
POSIX shells.

The original Python package which this work was inspired by can be found
at https://pypi.python.org/pypi/shellescape.
*/
package shellescape // "import gopkg.in/alessio/shellescape.v1"

/*
The functionality provided by shellescape.Quote could be helpful
in those cases where it is known that the output of a Go program will
be appended to/used in the context of shell programs' command line arguments.
*/

import (
	"regexp"
	"strings"
)

var pattern *regexp.Regexp

func init() {
	pattern = regexp.MustCompile(`[^\w@%+=:,./-]`)
}

// Quote returns a shell-escaped version of the string s. The returned value
// is a string that can safely be used as one token in a shell command line.
func Quote(s string) string {
	if len(s) == 0 {
		return "''"
	}
	if pattern.MatchString(s) {
		return "'" + strings.Replace(s, "'", "'\"'\"'", -1) + "'"
	}

	return s
}

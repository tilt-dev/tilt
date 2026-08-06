package logstore

import (
	"strings"

	"github.com/tilt-dev/tilt/pkg/model"
)

// Manifest names are right-aligned into a column this many bytes wide;
// longer names are truncated with an ellipsis.
const sourcePrefixMaxNameLen = 13

const sourcePrefixSeparator = " │ "
const sourcePrefixEllipsis = "…"

// The most bytes appendSourcePrefix can write, for pre-sizing builders.
const sourcePrefixReserveLen = sourcePrefixMaxNameLen +
	len(sourcePrefixSeparator) + len(sourcePrefixEllipsis)

// Writes SourcePrefix(n) into sb without allocating intermediate strings.
// This runs once per rendered log line, so it must not allocate.
func appendSourcePrefix(sb *strings.Builder, n model.ManifestName) {
	if n == "" || n == model.MainTiltfileManifestName {
		return
	}
	if len(n) > sourcePrefixMaxNameLen {
		sb.WriteString(string(n[:sourcePrefixMaxNameLen-1]))
		sb.WriteString(sourcePrefixEllipsis)
	} else {
		for i := len(n); i < sourcePrefixMaxNameLen; i++ {
			sb.WriteByte(' ')
		}
		sb.WriteString(string(n))
	}
	sb.WriteString(sourcePrefixSeparator)
}

func SourcePrefix(n model.ManifestName) string {
	if n == "" || n == model.MainTiltfileManifestName {
		return ""
	}
	sb := strings.Builder{}
	sb.Grow(sourcePrefixReserveLen)
	appendSourcePrefix(&sb, n)
	return sb.String()
}

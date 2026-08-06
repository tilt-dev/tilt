package logstore

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/pkg/model"
)

// Characterization of the prefix format: right-aligned in a 13-byte column,
// long names truncated with an ellipsis. appendSourcePrefix must stay
// byte-for-byte identical to SourcePrefix.
func TestSourcePrefix(t *testing.T) {
	cases := []struct {
		name     model.ManifestName
		expected string
	}{
		{"", ""},
		{model.MainTiltfileManifestName, ""},
		{"fe", "           fe │ "},
		{"exactly-13-ch", "exactly-13-ch │ "},
		{"a-name-that-is-too-long", "a-name-that-… │ "},
	}

	for _, c := range cases {
		t.Run(string(c.name), func(t *testing.T) {
			assert.Equal(t, c.expected, SourcePrefix(c.name))

			sb := strings.Builder{}
			appendSourcePrefix(&sb, c.name)
			assert.Equal(t, c.expected, sb.String())
		})
	}
}

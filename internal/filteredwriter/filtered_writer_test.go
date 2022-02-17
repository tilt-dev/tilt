package filteredwriter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilteredWriter(t *testing.T) {
	for _, tc := range []struct {
		name           string
		input          []string
		expectedOutput string
	}{
		{
			name:           "normal",
			input:          []string{"abc\n", "foobar\n", "def\n"},
			expectedOutput: "abc\ndef\n",
		},
		{
			name:           "all one line",
			input:          []string{"abc\nfoobar\ndef\n"},
			expectedOutput: "abc\ndef\n",
		},
		{
			name:           "lines split across writes",
			input:          []string{"ab", "c\n", "foo", "ba", "r\n", "de", "f", "\n"},
			expectedOutput: "abc\ndef\n",
		},
		{
			name: "actual warning we want to suppress",
			input: []string{
				"hello\n",
				"W1021 14:53:11.799222 68992 reflector.go:299] github.com/tilt-dev/tilt/internal/k8s/watch.go:172: " +
					"watch of *v1.Pod ended with: too old resource version: 191906663 (191912819)\n",
				"goodbye\n"},
			expectedOutput: "hello\ngoodbye\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := bytes.NewBuffer(nil)
			fw := New(out, func(s string) bool {
				return strings.Contains(s, "foobar") || strings.Contains(s, "too old resource version")
			})
			for _, s := range tc.input {
				_, err := fw.Write([]byte(s))
				require.NoError(t, err)
			}

			require.Equal(t, tc.expectedOutput, out.String())
		})
	}
}

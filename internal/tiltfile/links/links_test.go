package links

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaybeAddScheme(t *testing.T) {
	cases := []struct {
		name              string
		url               string
		expectErrContains string
		expectURL         string
	}{
		{
			name:      "preserves_scheme",
			url:       "ws://www.zombo.com",
			expectURL: "ws://www.zombo.com",
		},
		{
			name:      "adds_http_if_no_scheme",
			url:       "www.zombo.com",
			expectURL: "http://www.zombo.com",
		},
		{
			name:              "empty",
			url:               "",
			expectErrContains: "url empty",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual, err := maybeAddScheme(c.url)
			if c.expectErrContains != "" {
				require.Error(t, err, "expected error but got none")
				require.Contains(t, err.Error(), c.expectErrContains, "error did not contain expected message")
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.expectURL, actual, "expected URL != actual URL")
		})
	}
}

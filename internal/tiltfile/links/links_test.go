package links

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
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
			name:      "adds_http_to_host_port",
			url:       "localhost:8000",
			expectURL: "http://localhost:8000",
		},
		{
			name:      "adds_http_to_host_port_with_path",
			url:       "localhost:8000/v1/app",
			expectURL: "http://localhost:8000/v1/app",
		},
		{
			name:      "adds_http_to_ip_port",
			url:       "127.0.0.1:8000",
			expectURL: "http://127.0.0.1:8000",
		},
		{
			name:      "preserves_scheme_with_port",
			url:       "https://localhost:8000",
			expectURL: "https://localhost:8000",
		},
		{
			name:      "preserves_opaque_scheme",
			url:       "mailto:hello@zombo.com",
			expectURL: "mailto:hello@zombo.com",
		},
		{
			name:              "empty",
			url:               "",
			expectErrContains: "url empty",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual, err := parseAndMaybeAddScheme(c.url)
			if c.expectErrContains != "" {
				require.Error(t, err, "expected error but got none")
				require.Contains(t, err.Error(), c.expectErrContains, "error did not contain expected message")
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.expectURL, actual.String(), "expected URL != actual URL")
		})
	}
}

func TestLinkProps(t *testing.T) {
	f := starkit.NewFixture(t, NewPlugin())

	f.File("Tiltfile", `
l = link("localhost:4000", "web")
print(l.url)
print(l.name)
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:4000\nweb\n", f.PrintOutput())
}
func TestLinkPropsImmutable(t *testing.T) {
	f := starkit.NewFixture(t, NewPlugin())

	f.File("Tiltfile", `
l = link("localhost:4000", "web")
l.url = "XXX"
`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "can't assign to .url field of struct")
}

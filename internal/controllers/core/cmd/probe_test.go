package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestProbeFromSpecUnsupported(t *testing.T) {
	// empty probe spec
	probeSpec := &v1alpha1.Probe{}
	p, err := proberFromSpec(&FakeProberManager{}, probeSpec)
	assert.Nil(t, p)
	assert.ErrorIs(t, err, ErrUnsupportedProbeType)
}

func TestProbeFromSpecTCP(t *testing.T) {
	type tc struct {
		host        string
		port        int32
		expectedErr string
	}
	cases := []tc{
		{"localhost", 8080, ""},
		{"localhost", 0, "port number out of range: 0"},
		{"localhost", -1, "port number out of range: -1"},
		{"localhost", 65536, "port number out of range: 65536"},
		{"localhost", 1234, ""},
		{"", 22, ""},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("[%d] %s:%d", i, tc.host, tc.port), func(t *testing.T) {
			probeSpec := &v1alpha1.Probe{
				Handler: v1alpha1.Handler{
					TCPSocket: &v1alpha1.TCPSocketAction{
						Host: tc.host,
						Port: tc.port,
					},
				},
			}
			manager := &FakeProberManager{}
			p, err := proberFromSpec(manager, probeSpec)
			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr)
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, p)
				if tc.host != "" {
					assert.Equal(t, tc.host, manager.tcpHost)
				} else {
					assert.Equal(t, "localhost", manager.tcpHost)
				}
				assert.Equal(t, int(tc.port), int(manager.tcpPort))
			}
		})
	}
}

func TestProbeFromSpecHTTP(t *testing.T) {
	type tc struct {
		httpGet     *v1alpha1.HTTPGetAction
		expectedErr string
	}
	cases := []tc{
		{
			&v1alpha1.HTTPGetAction{
				Host: "",
				Port: 80,
			},
			"",
		},
		{
			&v1alpha1.HTTPGetAction{
				Host: "localhost",
				Port: -1,
			},
			"port number out of range: -1",
		},
		{
			&v1alpha1.HTTPGetAction{
				Host:   "localhost",
				Port:   8080,
				Scheme: v1alpha1.URISchemeHTTPS,
			},
			"",
		},
		{
			&v1alpha1.HTTPGetAction{
				Host: "localhost",
				Port: 8888,
				HTTPHeaders: []v1alpha1.HTTPHeader{
					{Name: "X-Fake-Header", Value: "value-1"},
					{Name: "X-Fake-Header", Value: "value-2"},
					{Name: "Content-Type", Value: "application/json"},
				},
			},
			"",
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("[%d] %v", i, tc.httpGet), func(t *testing.T) {
			probeSpec := &v1alpha1.Probe{
				Handler: v1alpha1.Handler{
					HTTPGet: tc.httpGet,
				},
			}
			manager := &FakeProberManager{}
			p, err := proberFromSpec(manager, probeSpec)
			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr)
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, p)
				u := manager.httpURL
				if tc.httpGet.Host != "" {
					assert.Equal(t, tc.httpGet.Host, u.Hostname())
				} else {
					assert.Equal(t, "localhost", u.Hostname())
				}
				assert.Equal(t, fmt.Sprintf("%d", tc.httpGet.Port), u.Port())
				if tc.httpGet.Scheme != "" {
					assert.Equal(t, strings.ToLower(string(tc.httpGet.Scheme)), u.Scheme)
				} else {
					assert.Equal(t, "http", u.Scheme)
				}
				assert.Equal(t, tc.httpGet.Path, u.Path)
				for _, h := range tc.httpGet.HTTPHeaders {
					assert.Contains(t, manager.httpHeaders[h.Name], h.Value)
				}
			}
		})
	}
}

func TestProbeFromSpecExec(t *testing.T) {
	cases := [][]string{
		{"echo"},
		{"echo", "arg1", "arg2"},
	}
	for i, command := range cases {
		t.Run(fmt.Sprintf("[%d] %s", i, command), func(t *testing.T) {
			probeSpec := &v1alpha1.Probe{
				Handler: v1alpha1.Handler{
					Exec: &v1alpha1.ExecAction{
						Command: command,
					},
				},
			}
			manager := &FakeProberManager{}
			p, err := proberFromSpec(manager, probeSpec)
			assert.Nil(t, err)
			assert.NotNil(t, p)
			assert.Equal(t, command[0], manager.execName)
			assert.Equal(t, command[1:], manager.execArgs)
		})
	}
}

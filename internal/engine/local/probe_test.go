package local

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestProbeFromSpecUnsupported(t *testing.T) {
	// empty probe spec
	probeSpec := &v1.Probe{}
	p, err := proberFromSpec(&FakeProberManager{}, probeSpec)
	assert.Nil(t, p)
	assert.ErrorIs(t, err, ErrUnsupportedProbeType)
}

func TestProbeFromSpecTCP(t *testing.T) {
	type tc struct {
		host        string
		port        intstr.IntOrString
		expectedErr string
	}
	cases := []tc{
		{"localhost", intstr.FromInt(8080), ""},
		{"localhost", intstr.IntOrString{}, "invalid port number: 0"},
		{"localhost", intstr.FromString("http"), "invalid port number: http"},
		{"localhost", intstr.FromInt(-1), "invalid port number: -1"},
		{"localhost", intstr.FromInt(65536), "invalid port number: 65536"},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("[%d] %s:%s", i, tc.host, tc.port.String()), func(t *testing.T) {
			probeSpec := &v1.Probe{
				Handler: v1.Handler{
					TCPSocket: &v1.TCPSocketAction{
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
				assert.Equal(t, tc.host, manager.tcpHost)
				assert.Equal(t, tc.port.IntValue(), manager.tcpPort)
			}
		})
	}
}

func TestProbeFromSpecHTTP(t *testing.T) {
	type tc struct {
		httpGet     *v1.HTTPGetAction
		expectedErr string
	}
	cases := []tc{
		{
			&v1.HTTPGetAction{
				Host: "localhost",
				Port: intstr.FromInt(80),
			},
			"",
		},
		{
			&v1.HTTPGetAction{
				Host: "localhost",
				Port: intstr.FromInt(-1),
			},
			"invalid port number: -1",
		},
		{
			&v1.HTTPGetAction{
				Host:   "localhost",
				Port:   intstr.FromInt(8080),
				Scheme: v1.URISchemeHTTPS,
			},
			"",
		},
		{
			&v1.HTTPGetAction{
				Host: "localhost",
				Port: intstr.FromInt(8888),
				HTTPHeaders: []v1.HTTPHeader{
					{Name: "X-Fake-Header", Value: "value-1"},
					{Name: "X-Fake-Header", Value: "value-2"},
					{Name: "Content-Type", Value: "application/json"},
				},
			},
			"",
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("[%d] %s", i, tc.httpGet.String()), func(t *testing.T) {
			probeSpec := &v1.Probe{
				Handler: v1.Handler{
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
				assert.Equal(t, tc.httpGet.Host, u.Hostname())
				assert.Equal(t, strconv.Itoa(tc.httpGet.Port.IntValue()), u.Port())
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
			probeSpec := &v1.Probe{
				Handler: v1.Handler{
					Exec: &v1.ExecAction{
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

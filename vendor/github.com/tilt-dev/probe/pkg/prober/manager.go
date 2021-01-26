package prober

import (
	"context"
	"net/http"
	"net/url"
)

// Manager creates standardized Prober instances from shared instances of underlying
// implementations for common probe types.
type Manager struct {
	httpProber HTTPGetProber
	execProber ExecProber
	tcpProber  TCPSocketProber
}

// NewManager creates a Manager instance to create standard Prober types.
func NewManager() *Manager {
	return &Manager{
		httpProber: NewHTTPGetProber(),
		execProber: NewExecProber(),
		tcpProber:  NewTCPSocketProber(),
	}
}

// HTTPGet returns a ProberFunc that performs HTTP GET probes for a specific URL & header combination.
func (m *Manager) HTTPGet(u *url.URL, headers http.Header) ProberFunc {
	return func(ctx context.Context) (Result, string, error) {
		return m.httpProber.Probe(ctx, u, headers)
	}
}

// TCPSocket returns a ProberFunc that performs TCP socket probes for a specific host & port combination.
func (m *Manager) TCPSocket(host string, port int) ProberFunc {
	return func(ctx context.Context) (Result, string, error) {
		return m.tcpProber.Probe(ctx, host, port)
	}
}

// Exec returns a ProberFunc that performs exec probes for a specific command.
func (m *Manager) Exec(name string, args ...string) ProberFunc {
	return func(ctx context.Context) (Result, string, error) {
		return m.execProber.Probe(ctx, name, args...)
	}
}

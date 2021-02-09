package local

import (
	"context"
	"net/http"
	"net/url"
	"sync/atomic"

	"github.com/tilt-dev/probe/pkg/prober"
)

func NewFakeProberManager() *FakeProberManager {
	return &FakeProberManager{}
}

type FakeProberManager struct {
	probeCount int32

	httpURL     *url.URL
	httpHeaders http.Header

	tcpHost string
	tcpPort int

	execName string
	execArgs []string
}

func (m *FakeProberManager) HTTPGet(u *url.URL, headers http.Header) prober.ProberFunc {
	m.httpURL = u
	m.httpHeaders = headers
	atomic.AddInt32(&m.probeCount, 1)
	return successProbe
}

func (m *FakeProberManager) TCPSocket(host string, port int) prober.ProberFunc {
	m.tcpHost = host
	m.tcpPort = port
	atomic.AddInt32(&m.probeCount, 1)
	return successProbe
}

func (m *FakeProberManager) Exec(name string, args ...string) prober.ProberFunc {
	m.execName = name
	m.execArgs = args
	atomic.AddInt32(&m.probeCount, 1)
	return successProbe
}

func (m *FakeProberManager) ProbeCount() int {
	return int(atomic.LoadInt32(&m.probeCount))
}

func successProbe(_ context.Context) (prober.Result, string, error) {
	return prober.Success, "fake probe succeeded!", nil
}

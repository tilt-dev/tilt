package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/httpstream"

	"github.com/tilt-dev/tilt/internal/testutils"
)

type fakeDialer struct {
	dialed             bool
	conn               httpstream.Connection
	err                error
	negotiatedProtocol string
}

func (d *fakeDialer) Dial(protocols ...string) (httpstream.Connection, string, error) {
	d.dialed = true
	return d.conn, d.negotiatedProtocol, d.err
}

var fakeNewPodDialer = newPodDialerFn(func(namespace Namespace, podID PodID) (httpstream.Dialer, error) {
	return &fakeDialer{}, nil
})

func TestPortForwardEmptyHost(t *testing.T) {
	ctx := testutils.LoggerCtx()
	client := portForwardClient{newPodDialer: fakeNewPodDialer}
	pf, err := client.CreatePortForwarder(ctx, "default", "podid", 8080, 8080, "")
	assert.NoError(t, err)
	assert.Equal(t, []string{"127.0.0.1", "::1"}, pf.Addresses())
}

func TestPortForwardLocalhost(t *testing.T) {
	ctx := testutils.LoggerCtx()
	client := portForwardClient{newPodDialer: fakeNewPodDialer}
	pf, err := client.CreatePortForwarder(ctx, "default", "podid", 8080, 8080, "localhost")
	assert.NoError(t, err)
	assert.Equal(t, []string{"127.0.0.1", "::1"}, pf.Addresses())
}

func TestPortForwardInvalidDomain(t *testing.T) {
	ctx := testutils.LoggerCtx()
	client := portForwardClient{newPodDialer: fakeNewPodDialer}
	_, err := client.CreatePortForwarder(ctx, "default", "podid", 8080, 8080, "domain.invalid")
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "failed to look up address for domain.invalid")
	}
}

func TestPortForwardAllHosts(t *testing.T) {
	ctx := testutils.LoggerCtx()
	client := portForwardClient{newPodDialer: fakeNewPodDialer}
	pf, err := client.CreatePortForwarder(ctx, "default", "podid", 8080, 8080, "0.0.0.0")
	assert.NoError(t, err)
	assert.Equal(t, []string{"0.0.0.0"}, pf.Addresses())
}

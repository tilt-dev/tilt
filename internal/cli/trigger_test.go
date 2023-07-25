package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"testing"

	"github.com/tilt-dev/tilt/internal/testutils"

	"github.com/stretchr/testify/require"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func TestTriggerSuccess(t *testing.T) {
	f := newTriggerFixture(t)
	streams, _, out, errOut := genericclioptions.NewTestIOStreams()
	cmd := newTriggerCmd(streams)
	c := cmd.register()
	err := c.Flags().Parse([]string{"foo"})
	require.NoError(t, err)
	err = cmd.run(f.ctx, c.Flags().Args())
	require.NoError(t, err)

	require.Equal(t, "Successfully triggered update for resource: \"foo\"\n", out.String())
	require.Equal(t, 0, errOut.Len())
}

func TestTriggerFailure(t *testing.T) {
	f := newTriggerFixture(t)
	f.responseBody = "nothing ever works"
	streams, _, out, errOut := genericclioptions.NewTestIOStreams()
	cmd := newTriggerCmd(streams)
	c := cmd.register()
	err := c.Flags().Parse([]string{"foo"})
	require.NoError(t, err)
	err = cmd.run(f.ctx, c.Flags().Args())
	require.Equal(t, "nothing ever works", err.Error())

	require.Equal(t, 0, errOut.Len())
	require.Equal(t, 0, out.Len())
}

func TestTriggerNotFound(t *testing.T) {
	f := newTriggerFixture(t)
	f.responseBody = "nothing ever works"
	f.responseStatus = http.StatusNotFound
	streams, _, out, errOut := genericclioptions.NewTestIOStreams()
	cmd := newTriggerCmd(streams)
	c := cmd.register()
	err := c.Flags().Parse([]string{"foo"})
	require.NoError(t, err)
	err = cmd.run(f.ctx, c.Flags().Args())
	require.Equal(t, "(404): nothing ever works", err.Error())

	require.Equal(t, 0, errOut.Len())
	require.Equal(t, 0, out.Len())
}

type triggerFixture struct {
	responseBody   string
	responseStatus int
	ctx            context.Context
}

func newTriggerFixture(t *testing.T) *triggerFixture {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	f := &triggerFixture{
		ctx:            ctx,
		responseStatus: http.StatusOK,
	}

	l, port := listenOnFreePort(t)
	origPort := defaultWebPort
	defaultWebPort = port
	t.Cleanup(func() {
		defaultWebPort = origPort
	})

	mux := &http.ServeMux{}
	mux.HandleFunc("/api/trigger", func(w http.ResponseWriter, req *http.Request) {
		http.Error(w, f.responseBody, f.responseStatus)
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", defaultWebPort),
		Handler: mux,
	}

	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() {
		_ = srv.Shutdown(ctx)
	})

	return f
}

func listenOnFreePort(t *testing.T) (net.Listener, int) {
	t.Helper()

	l, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	_, portString, err := net.SplitHostPort(l.Addr().String())
	require.NoError(t, err)

	port, err := strconv.Atoi(portString)
	require.NoError(t, err)
	return l, port
}

package cli

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/tilt-dev/tilt/internal/testutils"

	"github.com/phayes/freeport"
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
	require.Equal(t, errOut.Len(), 0)
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
	require.Equal(t, err.Error(), "nothing ever works")

	require.Equal(t, errOut.Len(), 0)
	require.Equal(t, out.Len(), 0)
}

type triggerFixture struct {
	responseBody string
	ctx          context.Context
}

func newTriggerFixture(t *testing.T) *triggerFixture {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()

	port, err := freeport.GetFreePort()
	require.NoError(t, err)
	origPort := defaultWebPort
	defaultWebPort = port
	t.Cleanup(func() {
		defaultWebPort = origPort
	})

	f := &triggerFixture{
		ctx: ctx,
	}

	mux := &http.ServeMux{}
	mux.HandleFunc("/api/trigger", func(w http.ResponseWriter, req *http.Request) {
		_, _ = fmt.Fprint(w, f.responseBody)
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", defaultWebPort),
		Handler: mux,
	}
	go func() { _ = srv.ListenAndServe() }()
	t.Cleanup(func() {
		_ = srv.Shutdown(ctx)
	})

	return f
}

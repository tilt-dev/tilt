package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/testdata"

	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/assets"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestUpdogGet(t *testing.T) {
	if true {
		t.Skip("TODO(nick): bring this back")
	}
	f := newServerFixture(t)
	defer f.TearDown()

	err := f.client.Create(f.ctx, &v1alpha1.Cmd{
		ObjectMeta: metav1.ObjectMeta{Name: "my-sleep"},
		Spec: v1alpha1.CmdSpec{
			Args: []string{"sleep", "1"},
		},
	})
	require.NoError(t, err)

	out := bytes.NewBuffer(nil)
	updogGet := newUpdogGetCmd()
	updogGet.register()
	updogGet.options.IOStreams.Out = out

	err = updogGet.run(f.ctx, []string{"cmd", "my-sleep"})
	require.NoError(t, err)

	assert.Contains(t, out.String(), `NAME       CREATED AT
my-sleep`)
}

type serverFixture struct {
	t      *testing.T
	ctx    context.Context
	cancel func()
	hudsc  *server.HeadsUpServerController
	client ctrlclient.Client

	origPort int
}

func newServerFixture(t *testing.T) *serverFixture {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)
	memconn := server.ProvideMemConn()
	port, err := freeport.GetFreePort()
	require.NoError(t, err)

	cfg, err := server.ProvideTiltServerOptions(ctx, model.TiltBuild{}, memconn, "corgi-charge", testdata.CertKey())
	require.NoError(t, err)

	hudsc := server.ProvideHeadsUpServerController("localhost",
		model.WebPort(port), cfg, &server.HeadsUpServer{}, assets.NewFakeServer(), model.WebURL{})
	st := store.NewTestingStore()
	require.NoError(t, hudsc.SetUp(ctx, st))

	scheme := v1alpha1.NewScheme()

	client, err := ctrlclient.New(cfg.GenericConfig.LoopbackClientConfig, ctrlclient.Options{Scheme: scheme})
	require.NoError(t, err)

	origPort := defaultWebPort
	defaultWebPort = port

	return &serverFixture{
		t:        t,
		ctx:      ctx,
		cancel:   cancel,
		hudsc:    hudsc,
		client:   client,
		origPort: origPort,
	}
}

func (f *serverFixture) TearDown() {
	f.hudsc.TearDown(f.ctx)
	f.cancel()
	defaultWebPort = f.origPort
}

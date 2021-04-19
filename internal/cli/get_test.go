package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/wmclient/pkg/dirs"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/testdata"

	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/assets"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestGet(t *testing.T) {
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
	get := newGetCmd()
	get.register()
	get.options.IOStreams.Out = out

	err = get.run(f.ctx, []string{"cmd", "my-sleep"})
	require.NoError(t, err)

	assert.Contains(t, out.String(), `NAME       CREATED AT
my-sleep`)
}

type serverFixture struct {
	*tempdir.TempDirFixture
	ctx    context.Context
	cancel func()
	hudsc  *server.HeadsUpServerController
	client ctrlclient.Client

	origPort int
}

func newServerFixture(t *testing.T) *serverFixture {
	f := tempdir.NewTempDirFixture(t)

	dir := dirs.NewTiltDevDirAt(f.Path())

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)
	memconn := server.ProvideMemConn()
	webPort, err := freeport.GetFreePort()
	require.NoError(t, err)
	apiPort, err := freeport.GetFreePort()
	require.NoError(t, err)

	cfg, err := server.ProvideTiltServerOptions(ctx, model.TiltBuild{}, memconn, "corgi-charge", testdata.CertKey(),
		server.APIServerPort(apiPort))
	require.NoError(t, err)

	webListener, err := server.ProvideWebListener("localhost", model.WebPort(webPort))
	require.NoError(t, err)

	cfgAccess := server.ProvideConfigAccess(dir)
	hudsc := server.ProvideHeadsUpServerController(cfgAccess, model.ProvideAPIServerName(model.WebPort(webPort)),
		webListener, cfg, &server.HeadsUpServer{}, assets.NewFakeServer(), model.WebURL{})
	st := store.NewTestingStore()
	require.NoError(t, hudsc.SetUp(ctx, st))

	scheme := v1alpha1.NewScheme()

	client, err := ctrlclient.New(cfg.GenericConfig.LoopbackClientConfig, ctrlclient.Options{Scheme: scheme})
	require.NoError(t, err)

	_ = os.Setenv("TILT_CONFIG", filepath.Join(f.Path(), "config"))

	origPort := defaultWebPort
	defaultWebPort = webPort

	return &serverFixture{
		TempDirFixture: f,
		ctx:            ctx,
		cancel:         cancel,
		hudsc:          hudsc,
		client:         client,
		origPort:       origPort,
	}
}

func (f *serverFixture) TearDown() {
	f.hudsc.TearDown(f.ctx)
	f.cancel()
	os.Unsetenv("TILT_CONFIG")
	defaultWebPort = f.origPort
}

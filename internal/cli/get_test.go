package cli

import (
	"context"
	"net"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/cmd/util"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/testdata"
	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/assets"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/wmclient/pkg/analytics"
	"github.com/tilt-dev/wmclient/pkg/dirs"
)

func TestGet(t *testing.T) {
	f := newServerFixture(t)

	err := f.client.Create(f.ctx, &v1alpha1.Cmd{
		ObjectMeta: metav1.ObjectMeta{Name: "my-sleep"},
		Spec: v1alpha1.CmdSpec{
			Args: []string{"sleep", "1"},
		},
	})
	require.NoError(t, err)

	streams, _, out, _ := genericclioptions.NewTestIOStreams()
	get := newGetCmd(streams)
	get.register()

	err = get.run(f.ctx, []string{"cmd", "my-sleep"})
	require.NoError(t, err)

	assert.Contains(t, out.String(), `NAME       CREATED AT
my-sleep`)
}

type staticConnProvider struct {
	l net.Listener
}

func newStaticConnProvider(t testing.TB) *staticConnProvider {
	l, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	return &staticConnProvider{l: l}
}

func (p *staticConnProvider) Dial(network, address string) (net.Conn, error) {
	return net.Dial(network, p.l.Addr().String())
}
func (p *staticConnProvider) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, network, p.l.Addr().String())
}
func (p *staticConnProvider) Listen(network, address string) (net.Listener, error) {
	return p.l, nil
}

type serverFixture struct {
	*tempdir.TempDirFixture
	ctx       context.Context
	cancel    func()
	hudsc     *server.HeadsUpServerController
	client    ctrlclient.Client
	analytics *analytics.MemoryAnalytics

	origPort int
}

func newServerFixture(t *testing.T) *serverFixture {
	f := tempdir.NewTempDirFixture(t)
	util.BehaviorOnFatal(func(err string, code int) {
		t.Fatal(err)
	})

	dir := dirs.NewTiltDevDirAt(f.Path())

	ctx, a, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)
	apiConnProvider := newStaticConnProvider(t)
	_, apiPortString, _ := net.SplitHostPort(apiConnProvider.l.Addr().String())
	apiPort, err := strconv.Atoi(apiPortString)
	require.NoError(t, err)
	cfg, err := server.ProvideTiltServerOptions(ctx, model.TiltBuild{}, apiConnProvider,
		"corgi-charge", testdata.CertKey(), server.APIServerPort(apiPort))
	require.NoError(t, err)

	webListener, err := server.ProvideWebListener("localhost", model.WebPort(0))
	require.NoError(t, err)

	_, webPortString, _ := net.SplitHostPort(webListener.Addr().String())
	webPort, err := strconv.Atoi(webPortString)
	require.NoError(t, err)

	cfgAccess := server.ProvideConfigAccess(dir)
	hudsc := server.ProvideHeadsUpServerController(cfgAccess, model.ProvideAPIServerName(model.WebPort(webPort)),
		webListener, cfg, &server.HeadsUpServer{}, assets.NewFakeServer(), model.WebURL{})
	st := store.NewTestingStore()
	require.NoError(t, hudsc.SetUp(ctx, st))

	scheme := v1alpha1.NewScheme()
	client, err := ctrlclient.New(cfg.GenericConfig.LoopbackClientConfig, ctrlclient.Options{Scheme: scheme})
	require.NoError(t, err)

	t.Setenv("TILT_CONFIG", filepath.Join(f.Path(), "config"))

	origPort := defaultWebPort
	defaultWebPort = webPort

	ret := &serverFixture{
		TempDirFixture: f,
		ctx:            ctx,
		cancel:         cancel,
		hudsc:          hudsc,
		client:         client,
		origPort:       origPort,
		analytics:      a,
	}

	t.Cleanup(ret.TearDown)
	return ret
}

func (f *serverFixture) TearDown() {
	f.hudsc.TearDown(f.ctx)
	f.cancel()
	defaultWebPort = f.origPort
	util.DefaultBehaviorOnFatal()
}

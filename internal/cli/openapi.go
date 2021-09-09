package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/apiserver"
	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/assets"
	"github.com/tilt-dev/tilt/pkg/model"
)

type openapiCmd struct {
	streams genericclioptions.IOStreams
}

var _ tiltCmd = &openapiCmd{}

func newOpenapiCmd() *openapiCmd {
	streams := genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr, In: os.Stdin}
	return &openapiCmd{streams: streams}
}

func (c *openapiCmd) name() model.TiltSubcommand { return "openapi" }

func (c *openapiCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "openapi",
		Short:   "Print the openapi spec of the current tilt binary",
		Example: "tilt dump openapi > swagger.json",
	}

	return cmd
}

func (c *openapiCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)

	cmdTags := engineanalytics.CmdTags(map[string]string{})
	a.Incr("cmd.openapi", cmdTags.AsMap())
	defer a.Flush(time.Second)

	hs, err := newHeadlessServer(ctx)
	if err != nil {
		return err
	}
	defer hs.tearDown(ctx)

	restConfig := hs.loopbackClientConfig
	trConfig, err := restConfig.TransportConfig()
	if err != nil {
		return err
	}

	tr, err := transport.New(trConfig)
	if err != nil {
		return err
	}

	httpClient := http.Client{Transport: tr}
	resp, err := httpClient.Get(restConfig.Host + "/openapi/v2")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var msg json.RawMessage
	err = json.NewDecoder(resp.Body).Decode(&msg)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(c.streams.Out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(msg)
}

type headlessServer struct {
	memconn              apiserver.ConnProvider
	hudsc                *server.HeadsUpServerController
	loopbackClientConfig *rest.Config
}

func newHeadlessServer(ctx context.Context) (*headlessServer, error) {
	memconn := server.ProvideMemConn()
	serverOptions, err := server.ProvideTiltServerOptionsForHeadless(ctx, memconn, tiltInfo())
	if err != nil {
		return nil, err
	}
	webListener, err := server.ProvideWebListener("localhost", 0)
	if err != nil {
		return nil, err
	}
	hudsc := server.ProvideHeadsUpServerController(
		nil, "tilt-headless", webListener, serverOptions,
		&server.HeadsUpServer{}, assets.NewFakeServer(), model.WebURL{})
	st := store.NewTestingStore()
	err = hudsc.SetUp(ctx, st)
	if err != nil {
		return nil, err
	}
	return &headlessServer{
		memconn:              memconn,
		hudsc:                hudsc,
		loopbackClientConfig: serverOptions.GenericConfig.LoopbackClientConfig,
	}, nil
}

func (hs *headlessServer) tearDown(ctx context.Context) {
	hs.hudsc.TearDown(ctx)
}

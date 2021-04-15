package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/client-go/transport"

	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/pkg/model"
)

type openapiCmd struct {
}

var _ tiltCmd = &openapiCmd{}

func newOpenapiCmd() *openapiCmd {
	return &openapiCmd{}
}

func (c *openapiCmd) name() model.TiltSubcommand { return "openapi" }

func (c *openapiCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "openapi",
		Short:   "Print the openapi spec of a running Tilt apiserver",
		Example: "tilt dump openapi > swagger.json",
	}

	addConnectServerFlags(cmd)

	return cmd
}

func (c *openapiCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)

	cmdTags := engineanalytics.CmdTags(map[string]string{})
	a.Incr("cmd.openapi", cmdTags.AsMap())
	defer a.Flush(time.Second)

	getter, err := wireClientGetter(ctx)
	if err != nil {
		return err
	}

	restConfig, err := getter.ToRESTConfig()
	if err != nil {
		return err
	}

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

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(msg)
}

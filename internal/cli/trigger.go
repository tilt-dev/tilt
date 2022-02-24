package cli

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tilt-dev/tilt/internal/analytics"
	analytics2 "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/pkg/model"
)

type triggerCmd struct {
	streams genericclioptions.IOStreams
}

var _ tiltCmd = &triggerCmd{}

func newTriggerCmd(streams genericclioptions.IOStreams) *triggerCmd {
	return &triggerCmd{
		streams: streams,
	}
}

func (t triggerCmd) name() model.TiltSubcommand {
	return "trigger"
}

func (t triggerCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trigger [RESOURCE_NAME]",
		Short: "Trigger an update for the specified resource",
		Long: `Trigger an update for the specified resource.

If the resource has Trigger Mode: Manual and has pending changes, this command will cause those pending changes to be applied.

Otherwise, this command will force a full rebuild.
`,
		Args: cobra.ExactArgs(1),
	}
	addConnectServerFlags(cmd)
	return cmd
}

func (t triggerCmd) run(ctx context.Context, args []string) error {
	resource := args[0]

	a := analytics.Get(ctx)
	a.Incr("cmd.trigger", make(analytics2.CmdTags))
	defer a.Flush(time.Second)

	// TODO(maia): this should probably be the triggerPayload struct, but seems
	//   like a lot of code to move over (to avoid import cycles) for one call.
	payload := []byte(fmt.Sprintf(`{"manifest_names":[%q], "build_reason": %d}`, resource, model.BuildReasonFlagTriggerCLI))

	r, status := apiPostJson("trigger", payload)

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return errors.Wrap(err, "error reading response from tilt api")
	}
	_ = r.Close()

	body := strings.TrimSpace(string(b))
	if status != http.StatusOK {
		return fmt.Errorf("(%d): %s", status, body)
	}
	if len(body) > 0 {
		return errors.New(body)
	}

	_, _ = fmt.Fprintf(t.streams.Out, "Successfully triggered update for resource: %q\n", resource)
	return nil
}

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/windmilleng/tilt/pkg/model"
)

func newTriggerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trigger [RESOURCE_NAME]",
		Short: "Trigger an update for the specified resource",
		Long: `Trigger an update for the specified resource.

If the resource has Trigger Mode: Manual and has pending changes, this command will cause those pending changes to be applied.

Otherwise, this command will force a full rebuild.
`,
		Args: cobra.ExactArgs(1),
		Run:  triggerUpdate,
	}
	cmd.Flags().IntVar(&webPort, "port", DefaultWebPort, "Port for the Tilt HTTP server")
	return cmd
}

func triggerUpdate(cmd *cobra.Command, args []string) {
	resource := args[0]

	// TODO(maia): this should probably be the triggerPayload struct, but seems
	//   like a lot of code to move over (to avoid import cycles) for one call.
	payload := []byte(fmt.Sprintf(`{"manifest_names":[%q], "build_reason": %d}`, resource, model.BuildReasonFlagTriggerCLI))

	body := apiPostJson(webPort, "trigger", payload)
	_ = body.Close()

	fmt.Printf("Successfully triggered update for resource: %q\n", resource)
}

package cli

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/spf13/cobra"
)

func newTriggerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trigger [RESOURCE_NAME]",
		Short: "trigger an update for the specified resource",
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
	url := fmt.Sprintf("http://localhost:%d/api/trigger", webPort)
	payload := []byte(fmt.Sprintf(`{"manifest_names":[%q]}`, resource))
	res, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		cmdFail(fmt.Errorf("Could not connect to Tilt at %s: %v", url, err))
	}

	if res.StatusCode != http.StatusOK {
		body := "<no response body>"
		b, err := ioutil.ReadAll(res.Body)
		if err != nil {
			cmdFail(fmt.Errorf("Error reading response body from %s: %v", url, err))
		}
		if string(b) != "" {
			body = string(b)
		}
		cmdFail(fmt.Errorf("Request to %s failed with status %q: %s", url, res.Status, body))
	}
}

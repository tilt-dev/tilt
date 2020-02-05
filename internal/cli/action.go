package cli

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/spf13/cobra"
)

func newActionCmd() *cobra.Command {
	result := &cobra.Command{
		Use:   "action",
		Short: "perform the sepcified action",
		Long: `Perform the specified action via a call to Tilt's API server.
`,
	}

	result.AddCommand(newTriggerCmd())

	return result
}

func newTriggerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trigger [RESOURCE_NAME]",
		Short: "trigger an update for the specified resource",
		Long: `Trigger an update for the specified resource.

If no file changes for the resource are pending, this will force a full rebuild.

If it is a manual resource with pending file changes, this will cause a build of those pending changes.
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
		body := "<no response body"
		b, _ := ioutil.ReadAll(res.Body)
		if string(b) != "" {
			body = string(b)
		}
		cmdFail(fmt.Errorf("Request to %s failed (status code %d): %s", url, res.StatusCode, body))
	}
}

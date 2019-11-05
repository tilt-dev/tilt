package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func newDumpCmd() *cobra.Command {
	result := &cobra.Command{
		Use:   "dump",
		Short: "dump internal Tilt state",
		Long: `Dumps internal Tilt state to stdout.

Intended to help Tilt developers inspect Tilt when things go wrong,
and figure out better ways to expose this info to Tilt users.

The format of the dump state does not make any API or compatibility promises,
and may change frequently.
`,
	}

	result.AddCommand(newDumpWebviewCmd())
	result.AddCommand(newDumpEngineCmd())

	return result
}

func newDumpWebviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webview",
		Short: "dump the state backing the webview",
		Long: `Dumps the state backing the webview to stdout.

The webview is the JSON used to render the React UX.

The format of the dump state does not make any API or compatibility promises,
and may change frequently.
`,
		Run: dumpWebview,
	}
	cmd.Flags().IntVar(&webPort, "port", DefaultWebPort, "Port for the Tilt HTTP server")
	return cmd
}

func newDumpEngineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "engine",
		Short: "dump the engine state",
		Long: `Dumps the state of the Tilt engine to stdout.

The engine state is the central store where Tilt keeps all information about
the build specification, build history, and deployed resources.

The format of the dump state does not make any API or compatibility promises,
and may change frequently.
`,
		Run: dumpEngine,
	}
	cmd.Flags().IntVar(&webPort, "port", DefaultWebPort, "Port for the Tilt HTTP server")
	return cmd
}

func cmdFail(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
	os.Exit(1)
}

func dumpWebview(cmd *cobra.Command, args []string) {
	url := fmt.Sprintf("http://localhost:%d/api/view", webPort)
	res, err := http.Get(url)
	if err != nil {
		cmdFail(fmt.Errorf("Could not connect to Tilt at %s: %v", url, err))
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != http.StatusOK {
		cmdFail(fmt.Errorf("Error connecting to Tilt at %s: %d", url, res.StatusCode))
	}

	err = dumpJSON(res.Body)
	if err != nil {
		cmdFail(fmt.Errorf("dump webview: %v", err))
	}
}

func dumpEngine(cmd *cobra.Command, args []string) {
	url := fmt.Sprintf("http://localhost:%d/api/dump/engine", webPort)
	res, err := http.Get(url)
	if err != nil {
		cmdFail(fmt.Errorf("Could not connect to Tilt at %s: %v", url, err))
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != http.StatusOK {
		cmdFail(fmt.Errorf("Error connecting to Tilt at %s: %d", url, res.StatusCode))
	}

	err = dumpJSON(res.Body)
	if err != nil {
		cmdFail(fmt.Errorf("dump engine: %v", err))
	}
}

func dumpJSON(reader io.Reader) error {
	decoder := json.NewDecoder(reader)

	var result interface{}
	err := decoder.Decode(&result)
	if err != nil {
		return errors.Wrap(err, "Could not decode")
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(result)
	if err != nil {
		return errors.Wrap(err, "Could not print")
	}
	return nil
}

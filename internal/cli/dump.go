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
	result.AddCommand(newDumpLogStoreCmd())

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

Excludes logs.
`,
		Run: dumpEngine,
	}
	cmd.Flags().IntVar(&webPort, "port", DefaultWebPort, "Port for the Tilt HTTP server")
	return cmd
}

func newDumpLogStoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logstore",
		Short: "dump the log store",
		Long: `Dumps the state of the Tilt log store to stdout.

Every log of a Tilt-managed resource is aggregated into a central structured log
store before display. Dumps the JSON representation of this store.

The format of the dump state does not make any API or compatibility promises,
and may change frequently.
`,
		Run: dumpLogStore,
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

	result, err := decodeJSON(res.Body)
	if err != nil {
		cmdFail(fmt.Errorf("dump engine: %v", err))
	}

	obj, ok := result.(map[string]interface{})
	if ok {
		delete(obj, "LogStore")
	}

	err = encodeJSON(obj)
	if err != nil {
		cmdFail(fmt.Errorf("dump engine: %v", err))
	}
}

func dumpLogStore(cmd *cobra.Command, args []string) {
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

	result, err := decodeJSON(res.Body)
	if err != nil {
		cmdFail(fmt.Errorf("dump LogStore: %v", err))
	}

	var logStore interface{}
	obj, ok := result.(map[string]interface{})
	if ok {
		logStore, ok = obj["LogStore"]
	}

	if !ok {
		cmdFail(fmt.Errorf("No LogStore in engine: %v", err))
	}

	err = encodeJSON(logStore)
	if err != nil {
		cmdFail(fmt.Errorf("dump LogStore: %v", err))
	}
}

func dumpJSON(reader io.Reader) error {
	result, err := decodeJSON(reader)
	if err != nil {
		return err
	}
	return encodeJSON(result)
}

func decodeJSON(reader io.Reader) (interface{}, error) {
	decoder := json.NewDecoder(reader)

	var result interface{}
	err := decoder.Decode(&result)
	if err != nil {
		return nil, errors.Wrap(err, "Could not decode")
	}
	return result, err
}

func encodeJSON(result interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	err := encoder.Encode(result)
	if err != nil {
		return errors.Wrap(err, "Could not print")
	}
	return nil
}

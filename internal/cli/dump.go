package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/pkg/model"
)

func newDumpCmd(rootCmd *cobra.Command) *cobra.Command {
	result := &cobra.Command{
		Use:   "dump",
		Short: "Dump internal Tilt state",
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
	result.AddCommand(newDumpCliDocsCmd(rootCmd))
	result.AddCommand(newDumpImageDeployRefCmd())
	addCommand(result, newOpenapiCmd())

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
		Run:  dumpWebview,
		Args: cobra.NoArgs,
	}
	addConnectServerFlags(cmd)
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
		Run:  dumpEngine,
		Args: cobra.NoArgs,
	}
	addConnectServerFlags(cmd)
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
		Run:  dumpLogStore,
		Args: cobra.NoArgs,
	}
	addConnectServerFlags(cmd)
	return cmd
}

type dumpCliDocsCmd struct {
	rootCmd *cobra.Command
	dir     string
}

func newDumpCliDocsCmd(rootCmd *cobra.Command) *cobra.Command {
	c := &dumpCliDocsCmd{rootCmd: rootCmd}

	cmd := &cobra.Command{
		Use:   "cli-docs",
		Short: "Dumps markdown docs of the CLI",
		Args:  cobra.NoArgs,
		Run:   c.run,
	}
	cmd.Flags().StringVar(&c.dir, "dir", ".", "The directory to dump to")
	return cmd
}

func (c *dumpCliDocsCmd) filePrepender(path string) string {
	return `---
title: Tilt CLI Reference
layout: docs
hideEditButton: true
---
`
}

func (c *dumpCliDocsCmd) linkHandler(link string) string {
	if strings.HasSuffix(link, ".md") {
		return strings.TrimSuffix(link, ".md") + ".html"
	}
	return link
}

func (c *dumpCliDocsCmd) run(cmd *cobra.Command, args []string) {
	err := doc.GenMarkdownTreeCustom(c.rootCmd, c.dir, c.filePrepender, c.linkHandler)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error generating CLI docs: %v", err)
		os.Exit(1)
	}
}

func newDumpImageDeployRefCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "image-deploy-ref REF",
		Short: "Determine the name and tag with which Tilt will deploy the given image",
		Long: `Determine the name and tag with which Tilt will deploy the given image.

This command is intended to be used with custom_build scripts.

Once the custom_build script has built the image at $EXPECTED_REF, it can
invoke:

echo $(tilt dump image-deploy-ref $EXPECTED_REF)

to print the deploy ref of the image. Tilt will read the image contents,
determine its hash, and create a content-based tag.

More info on custom build scripts: https://docs.tilt.dev/custom_build.html
`,
		Example: "tilt dump image-deploy-ref $EXPECTED_REF",
		Run:     dumpImageDeployRef,
		Args:    cobra.ExactArgs(1),
	}
}

func dumpImageDeployRef(cmd *cobra.Command, args []string) {
	ctx, cleanup := preCommand(context.Background(), "dump")
	defer func() {
		_ = cleanup()
	}()

	deps, err := wireDumpImageDeployRefDeps(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Initialization error: %v\n", err)
		os.Exit(1)
	}

	// Assume that people with complex custom_build commands are using
	// the kubernetes orchestrator.
	deps.DockerClient.SetOrchestrator(model.OrchestratorK8s)
	ref, err := deps.DockerBuilder.DumpImageDeployRef(ctx, args[0])
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s", container.FamiliarString(ref))
}

func dumpWebview(cmd *cobra.Command, args []string) {
	body := apiGet("view")

	err := dumpJSON(body)
	if err != nil {
		cmdFail(fmt.Errorf("dump webview: %v", err))
	}
}

func dumpEngine(cmd *cobra.Command, args []string) {
	body := apiGet("dump/engine")
	defer func() {
		_ = body.Close()
	}()

	result, err := decodeJSON(body)
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
	body := apiGet("dump/engine")
	defer func() {
		_ = body.Close()
	}()

	result, err := decodeJSON(body)
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

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/mattn/go-tty"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/internal/snapshots"
	proto_webview "github.com/tilt-dev/tilt/pkg/webview"
)

func newSnapshotCmd() *cobra.Command {
	result := &cobra.Command{
		Use: "snapshot",
	}

	result.AddCommand(newViewCommand())
	result.AddCommand(newCreateSnapshotCommand())

	return result
}

type serveCmd struct {
	noOpen bool
}

func newViewCommand() *cobra.Command {
	c := &serveCmd{}
	result := &cobra.Command{
		Use:   "view <path/to/snapshot.json>",
		Short: "Serves the specified snapshot file and optionally opens it in the browser",
		Long:  "Serves the specified snapshot file and optionally opens it in the browser",
		Example: `
# Run tilt ci and save a snapshot
tilt ci --output-snapshot-on-exit=snapshot.json
# View that snapshot
tilt snapshot view snapshot.json

# Or pipe the snapshot to stdin and specify the snapshot as '-'
curl http://myci.com/path/to/snapshot | tilt snapshot view -
`,
		Args: cobra.ExactArgs(1),
		Run:  c.run,
	}

	result.Flags().BoolVar(&c.noOpen, "no-open", false, "Do not automatically open the snapshot in the browser")
	addStartSnapshotViewServerFlags(result)

	return result
}

// blocks until any key is pressed or ctx is canceled
func waitForKey(ctx context.Context) error {
	t, err := tty.Open()
	if err != nil {
		return err
	}
	defer func() { _ = t.Close() }()

	done := make(chan struct{})
	errCh := make(chan error)

	go func() {
		_, err = t.ReadRune()
		if err != nil {
			errCh <- err
			return
		}
		close(done)
	}()

	select {
	case <-ctx.Done():
		return nil
	case <-done:
		return nil
	case err := <-errCh:
		return err
	}
}

func (c *serveCmd) run(_ *cobra.Command, args []string) {
	err := c.serveSnapshot(args[0])
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v", err)
		os.Exit(1)
	}
}

func readSnapshot(snapshotArg string) ([]byte, error) {
	var r io.Reader
	if snapshotArg == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(snapshotArg)
		if err != nil {
			return nil, err
		}
		r = f
		defer func() { _ = f.Close() }()
	}

	return io.ReadAll(r)
}

func (c *serveCmd) serveSnapshot(snapshotPath string) error {
	ctx := preCommand(context.Background(), "snapshot view")
	a := analytics.Get(ctx)
	cmdTags := engineanalytics.CmdTags(map[string]string{})
	a.Incr("cmd.snapshot.view", cmdTags.AsMap())
	defer a.Flush(time.Second)

	host := provideWebHost()
	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, snapshotViewPortFlag))
	if err != nil {
		return fmt.Errorf("could not get a free port: %w", err)
	}
	defer l.Close()

	port := l.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://%s:%d/snapshot/local",
		strings.Replace(string(host), "0.0.0.0", "127.0.0.1", 1),
		port)

	fmt.Printf("Serving snapshot at %s\n", url)

	wg, ctx := errgroup.WithContext(ctx)
	wg.Go(func() error {
		snapshot, err := readSnapshot(snapshotPath)
		if err != nil {
			return err
		}
		return snapshots.Serve(ctx, l, snapshot)
	})

	// give the server a little bit of time to spin up
	time.Sleep(200 * time.Millisecond)

	if !c.noOpen {
		err := browser.OpenURL(url)
		if err != nil {
			return err
		}
	}

	keyPressed := errors.New("pressed key to exit")
	wg.Go(func() error {
		fmt.Println("Press any key to exit")
		err := waitForKey(ctx)
		if err != nil {
			return err
		}
		return keyPressed
	})

	err = wg.Wait()
	if err != nil && err != keyPressed {
		return err
	}

	return nil
}

func newCreateSnapshotCommand() *cobra.Command {
	result := &cobra.Command{
		Use:   "create [file to save]",
		Short: "Creates a snapshot file from a currently running Tilt instance",
		Long:  "Creates a snapshot file that can be viewed with `tilt snapshot view`",
		Example: `
tilt snapshot create snapshot.json
# or if no file is specified, it goes to stdout
tilt snapshot create > snapshot.json

# to view the snapshot
tilt snapshot view snapshot.json
`,
		Args: cobra.MaximumNArgs(1),
		Run:  createSnapshot,
	}

	addConnectServerFlags(result)

	return result
}

func createSnapshot(cmd *cobra.Command, args []string) {
	body := apiGet("view")

	snapshot := proto_webview.Snapshot{
		View:      &proto_webview.View{},
		CreatedAt: timestamppb.Now(),
	}

	jsEncoder := &runtime.JSONPb{}
	err := jsEncoder.NewDecoder(body).Decode(&snapshot.View)
	if err != nil {
		cmdFail(fmt.Errorf("error reading snapshot from tilt: %v", err))
	}

	out := os.Stdout
	if len(args) > 0 {
		out, err = os.Create(args[0])
		if err != nil {
			cmdFail(fmt.Errorf("error creating %s: %v", args[0], err))
		}
	}

	err = jsEncoder.NewEncoder(out).Encode(&snapshot)
	if err != nil {
		cmdFail(fmt.Errorf("error serializing snapshot: %v", err))
	}
}

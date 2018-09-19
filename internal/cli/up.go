package cli

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/opentracing/opentracing-go"
	"github.com/spf13/cobra"
	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/output"
	"github.com/windmilleng/tilt/internal/tiltfile"
	"github.com/windmilleng/tilt/internal/tracer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type upCmd struct {
	watch       bool
	browserMode engine.BrowserMode
	cleanUpFn   func() error
	traceTags   string
}

func (c *upCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up <name>",
		Short: "stand up a manifest",
		Args:  cobra.ExactArgs(1),
	}

	cmd.Flags().BoolVar(&c.watch, "watch", false, "any started manifests will be automatically rebuilt and redeployed when files in their repos change")
	cmd.Flags().Var(&c.browserMode, "browser", "open a browser when the manifest first starts")
	cmd.Flags().StringVar(&c.traceTags, "traceTags", "", "tags to add to spans for easy querying, of the form: key1=val1,key2=val2")

	return cmd
}

func (c *upCmd) run(args []string) error {
	analyticsService.Incr("cmd.up", map[string]string{"watch": fmt.Sprintf("%v", c.watch)})
	defer analyticsService.Flush(time.Second)

	span := opentracing.StartSpan("Up")
	tags := tracer.TagStrToMap(c.traceTags)
	for k, v := range tags {
		span.SetTag(k, v)
	}

	l := logger.NewLogger(logLevel(), os.Stdout)
	ctx := output.WithOutputter(
		logger.WithLogger(
			opentracing.ContextWithSpan(context.Background(), span),
			l),
		output.NewOutputter(l))

	cleanUp := func() {
		span.Finish()
		err := c.cleanUpFn()
		if err != nil {
			l.Infof("error cleaning up: %v", err)
		}
	}
	defer cleanUp()

	// SIGNAL TRAPPING
	ctx, cancel := context.WithCancel(ctx)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		_ = <-sigs

		// Clean up anything that needs cleaning up
		cleanUp()

		// We rely on context cancellation being handled elsewhere --
		// otherwise there's no way to SIGINT/SIGTERM this app o_0
		cancel()
	}()

	logOutput(fmt.Sprintf("Starting Tilt (built %s)…\n", buildDateStamp()))

	tf, err := tiltfile.Load(tiltfile.FileName, os.Stdout)
	if err != nil {
		return err
	}

	manifestName := args[0]
	manifests, err := tf.GetManifestConfigs(manifestName)
	if err != nil {
		return err
	}

	manifestCreator, err := wireManifestCreator(ctx, c.browserMode)
	if err != nil {
		return err
	}

	err = manifestCreator.CreateManifests(ctx, manifests, c.watch)
	s, ok := status.FromError(err)
	if ok && s.Code() == codes.Unknown {
		return errors.New(s.Message())
	} else if err != nil {
		if err == context.Canceled {
			// Expected case, no need to be loud about it, just exit
			return nil
		}
		return err
	}

	return nil
}

func logOutput(s string) {
	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
	log.Printf(color.GreenString(s))
}

// Returns a build datestamp in the format 2018-08-30
func buildDateStamp() string {
	// TODO(nick): Add a mechanism to encode the datestamp in the binary with
	// ldflags. This currently only works if you are building your own
	// binaries. It won't work once we're distributing pre-built binaries.
	path, err := os.Executable()
	if err != nil {
		return "[unknown]"
	}

	info, err := os.Stat(path)
	if err != nil {
		return "[unknown]"
	}

	modTime := info.ModTime()
	return modTime.Format("2006-01-02")
}

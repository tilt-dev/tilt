package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"

	"github.com/opentracing/opentracing-go"
	"github.com/spf13/cobra"
	"github.com/windmilleng/tilt/internal/hud/proto"
	"github.com/windmilleng/tilt/internal/network"
	"github.com/windmilleng/tilt/internal/tracer"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

type hudCmd struct {
	traceTags string
}

func (c *hudCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hud",
		Short: "display Tilt/k8s status at a glance",
	}

	cmd.Flags().StringVar(&c.traceTags, "traceTags", "", "tags to add to spans for easy querying, of the form: key1=val1,key2=val2")

	return cmd
}

func (c *hudCmd) run(ctx context.Context, args []string) error {
	analyticsService.Incr("cmd.hud", nil)
	defer analyticsService.Flush(time.Second)

	span, ctx := opentracing.StartSpanFromContext(ctx, "Up")
	defer span.Finish()

	tags := tracer.TagStrToMap(c.traceTags)
	for k, v := range tags {
		span.SetTag(k, v)
	}

	logOutput(fmt.Sprintf("Starting the HUD (built %s)…\n", buildDateStamp()))

	return connectHud(ctx)
}

func connectHud(ctx context.Context) error {
	tty, err := curTty(ctx)
	if err != nil {
		return err
	}

	socketPath, err := proto.LocateSocket()
	if err != nil {
		return err
	}

	conn, err := grpc.Dial(
		socketPath,
		grpc.WithInsecure(),
		grpc.WithDialer(network.UnixDial),
	)
	if err != nil {
		return err
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			log.Printf("Error closing connection to HUD server: %v", err)
		}
	}()

	cli := proto.NewHudClient(conn)

	stream, err := cli.ConnectHud(ctx)

	if err != nil {
		log.Printf("Waiting for `tilt up` to start.")

		for {
			stream, err = cli.ConnectHud(ctx)
			if err == nil {
				break
			}
			select {
			case <-ctx.Done():
				err := ctx.Err()
				if err != context.Canceled {
					return err
				} else {
					return nil
				}
			case <-time.After(time.Second):
			}
		}
	}

	// TODO(maia): wrap in adaptors so we don't need to muck around in proto code
	if err := stream.Send(
		&proto.HudControl{
			Control: &proto.HudControl_Connect{Connect: &proto.ConnectRequest{
				TtyPath: tty,
			}},
		}); err != nil {
		return errors.Wrap(err, "error sending to hud server")
	}

	// TODO(matt) figure out why tcell's Fini isn't working for us here
	// this is a crummy workaround
	defer func() {
		cmd := exec.Command("reset")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			fmt.Printf("error restoring terminal settings: %v", err)
		}
	}()

	// Wait for the stream to close
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		// Returns when the stream closes (with error or otherwise)
		return errors.Wrap(stream.RecvMsg(nil), "error received from hud server")
	})

	// Forward any SIGWINCH's
	g.Go(func() error {
		winchCh := make(chan os.Signal, 10) // 10 is enough that we don't overflow the buffer
		signal.Notify(winchCh, syscall.SIGWINCH)
		defer signal.Stop(winchCh)
		for {
			select {
			case <-winchCh:
				if err := stream.Send(
					&proto.HudControl{
						Control: &proto.HudControl_WindowChange{WindowChange: &proto.WindowChange{}},
					}); err != nil {
					return err
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	return g.Wait()
}

func curTty(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "tty")
	cmd.Stdin = os.Stdin
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	outputString := strings.TrimRight(string(output), "\n")
	return outputString, nil
}

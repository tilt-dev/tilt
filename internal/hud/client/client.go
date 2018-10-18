package client

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/hud/proto"
	"github.com/windmilleng/tilt/internal/network"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

func ConnectHud(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

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
		err := stream.RecvMsg(nil)
		cancel()
		if err == io.EOF {
			return nil
		} else {
			return err
		}
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

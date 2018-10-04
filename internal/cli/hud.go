package cli

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/spf13/cobra"
	"github.com/windmilleng/tilt/internal/hud/proto"
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

	return doHud(ctx)
}

func doHud(ctx context.Context) error {
	fmt.Printf("hello hud\n")

	tty, err := curTty(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Got tty: %s\n", tty)

	socketPath, err := proto.LocateSocket()
	if err != nil {
		return err
	}

	conn, err := grpc.Dial(
		socketPath,
		grpc.WithInsecure(),
		grpc.WithDialer(UnixDial),
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	cli := proto.NewHudClient(conn)

	stream, err := cli.ConnectHud(ctx)
	if err != nil {
		return err
	}

	// TODO(maia): wrap in adaptors so we don't need to muck around in proto code
	if err := stream.Send(
		&proto.HudControl{
			Control: &proto.HudControl_Connect{Connect: &proto.ConnectRequest{
				TtyPath: tty,
			}},
		}); err != nil {
		return err
	}

	// Wait for the stream to close
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		// Returns when the stream closes (with error or otherwise)
		return stream.RecvMsg(nil)
	})

	// Forward any SIGWINCH's
	g.Go(func() error {
		winchCh := make(chan os.Signal, 10) // 10 is enough that we don't overflow the buffer
		signal.Notify(winchCh, syscall.SIGWINCH)
		defer signal.Stop(winchCh)
		for {
			select {
			case <-winchCh:
				log.Printf("sending winch")
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

func UnixDial(addr string, timeout time.Duration) (net.Conn, error) {
	// TODO(dbentley): do timeouts right
	return net.DialTimeout("unix", addr, 100*time.Millisecond)
}

func sendFDs(fdListener *net.UnixListener, fs []*os.File) error {
	if true {
		return nil
	}
	defer fdListener.Close()
	fdConn, err := fdListener.AcceptUnix()
	log.Printf("got a client who wants fds %v", fdConn)
	if err != nil {
		return err
	}
	connFd, err := fdConn.File()
	if err != nil {
		return err
	}
	defer fdConn.Close()

	fds := make([]int, len(fs))
	for i, f := range fs {
		fds[i] = int(f.Fd())
	}

	rights := syscall.UnixRights(fds...)
	return syscall.Sendmsg(int(connFd.Fd()), nil, rights, nil, 0)
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

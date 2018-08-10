package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/windmilleng/tilt/internal/proto"
	"github.com/windmilleng/tilt/internal/tiltd"
	"github.com/windmilleng/tilt/internal/tiltd/tiltd_server"
	"google.golang.org/grpc"
	"log"
	"net"
)

type daemonCmd struct{}

func (c *daemonCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "daemon",
		Short:  "start the daemon",
		Hidden: true,
	}

	return cmd
}

func (c *daemonCmd) run(args []string) error {
	addr := fmt.Sprintf("127.0.0.1:%d", tiltd.Port)
	log.Printf("Running tiltd listening on %s", addr)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	d, err := tiltd_server.NewDaemon()
	if err != nil {
		log.Fatalf("failed to make tiltd: %v", err)
	}

	s := grpc.NewServer()

	proto.RegisterDaemonServer(s, proto.NewGRPCServer(d))

	err = s.Serve(l)
	if err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
	return nil
}

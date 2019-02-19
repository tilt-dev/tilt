package cli

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/opentracing/opentracing-go"
	"github.com/spf13/cobra"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/options"
	"github.com/windmilleng/tilt/internal/synclet"
	"github.com/windmilleng/tilt/internal/synclet/proto"
	"github.com/windmilleng/tilt/internal/tracer"
	"google.golang.org/grpc"
)

type SyncletCmd struct {
	port    int
	debug   bool
	verbose bool
}

func (s *SyncletCmd) Register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "synclet",
		Short: "starts the tilt synclet daemon",
		Run: func(c *cobra.Command, args []string) {
			s.run()
		},
	}

	cmd.Flags().BoolVarP(&s.debug, "debug", "d", false, "Enable debug logging")
	cmd.Flags().BoolVarP(&s.verbose, "verbose", "v", false, "Enable verbose logging")
	cmd.Flags().IntVar(&s.port, "port", synclet.Port, "Server port")

	return cmd
}

func (sc *SyncletCmd) run() {
	ctx := logger.WithLogger(
		context.Background(),
		logger.NewLogger(logLevel(sc.verbose, sc.debug), os.Stdout))

	closer, err := tracer.Init(ctx)
	if err != nil {
		log.Fatalf("error initializing tracer: %v", err)
	}
	defer func() {
		err := closer()
		if err != nil {
			log.Fatalf("error closing tracer: %v", err)
		}
	}()

	addr := fmt.Sprintf("127.0.0.1:%d", sc.port)
	log.Printf("Running synclet listening on %s", addr)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// TODO(matt) figure out how to reconcile this with opt-in tracing
	t := opentracing.GlobalTracer()

	opts := options.MaxMsgServer()
	opts = append(opts, options.TracingInterceptorsServer(t)...)

	serv := grpc.NewServer(opts...)

	// TODO(Matt) fix this so either we don't need an k8s env to instantiate a synclet, or
	// so that we can still detect env inside of containers w/o kubectl
	// TODO(nick): Also fix this to detect the container runtime inside k8s.
	s, err := synclet.WireSynclet(ctx, k8s.EnvNone, container.RuntimeDocker)
	if err != nil {
		log.Fatalf("failed to wire synclet: %v", err)
	}

	proto.RegisterSyncletServer(serv, synclet.NewGRPCServer(s))

	err = serv.Serve(l)
	if err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

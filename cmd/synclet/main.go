package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/windmilleng/tilt/internal/k8s"

	"github.com/windmilleng/tilt/internal/synclet"
	"github.com/windmilleng/tilt/internal/synclet/proto"
	"google.golang.org/grpc"
)

var port = flag.Int("port", synclet.Port, "The server port")

func main() {
	ctx := context.Background()
	flag.Parse()
	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	log.Printf("Running synclet listening on %s", addr)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	serv := grpc.NewServer()

	// TODO(Matt) fix this so either we don't need an k8s env to instantiate a synclet, or
	// so that we can still detect env inside of containers w/o kubectl
	s, err := synclet.WireSynclet(ctx, k8s.EnvUnknown)
	if err != nil {
		log.Fatalf("failed to wire synclet: %v", err)
	}

	proto.RegisterSyncletServer(serv, synclet.NewGRPCServer(s))

	err = serv.Serve(l)
	if err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

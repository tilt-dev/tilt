package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

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

	s, err := synclet.WireSynclet(ctx)
	if err != nil {
		log.Fatalf("failed to wire synclet: %v", err)
	}

	proto.RegisterSyncletServer(serv, synclet.NewGRPCServer(s))

	err = serv.Serve(l)
	if err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

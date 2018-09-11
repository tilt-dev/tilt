package main

import (
	"flag"
	"fmt"
	"github.com/windmilleng/tilt/internal/synclet"
	"github.com/windmilleng/tilt/internal/synclet/proto"
	"google.golang.org/grpc"
	"log"
	"net"
)

var port = flag.Int("port", synclet.Port, "The server port")

func main() {
	flag.Parse()
	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	log.Printf("Running synclet listening on %s", addr)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()

	proto.RegisterSyncletServer(s, proto.NewGRPCServer(&synclet.Synclet{}))

	err = s.Serve(l)
	if err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	_ "github.com/windmilleng/tesseract/pkg/tracer"
	"github.com/windmilleng/tilt/internal/daemon"
	"github.com/windmilleng/tilt/internal/proto"
	"google.golang.org/grpc"
)

func main() {
	flag.Parse()
	addr := fmt.Sprintf("127.0.0.1:%d", daemon.Port)
	log.Printf("Running daemon listening on %s", addr)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	d, err := daemon.NewDaemon()
	if err != nil {
		log.Fatalf("failed to make daemon: %v", err)
	}

	s := grpc.NewServer()

	proto.RegisterDaemonServer(s, daemon.NewGRPCServer(d))

	err = s.Serve(l)
	if err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

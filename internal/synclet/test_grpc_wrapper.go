package synclet

import (
	"context"
	"log"
	"net"
	"path/filepath"
	"time"

	"google.golang.org/grpc"

	"github.com/windmilleng/tilt/internal/synclet/proto"
)

func FakeGRPCWrapper(ctx context.Context, c SyncletClient, tempDir string) (SyncletClient, error) {
	socket := filepath.Join(tempDir, "grpcSyncletSocket")
	l, err := net.Listen("unix", socket)
	if err != nil {
		return nil, err
	}

	dial, err := grpc.Dial(socket, grpc.WithInsecure(), grpc.WithDialer(unixDial))
	if err != nil {
		return nil, err
	}

	client := NewGRPCClient(dial)
	server := NewGRPCServer(c)

	grpcServer := grpc.NewServer()
	proto.RegisterSyncletServer(grpcServer, server)

	go func() {
		err := grpcServer.Serve(l)
		if err != nil && err != context.Canceled {
			log.Printf("FakeGRPCWrapper: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()
	return client, nil
}

func unixDial(addr string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", addr, timeout)
}

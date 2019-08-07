package synclet

import (
	"context"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"

	"github.com/windmilleng/tilt/internal/synclet/proto"
)

func FakeGRPCWrapper(ctx context.Context, c *TestSyncletClient) (SyncletClient, error) {
	socketDir, err := ioutil.TempDir("", "grpc")
	if err != nil {
		return nil, err
	}

	socket := filepath.Join(socketDir, "socket")
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
		_ = l.Close()
		_ = os.Remove(socket)
	}()
	return client, nil
}

func unixDial(addr string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", addr, timeout)
}

package hud

import (
	"context"
	"fmt"
	"log"

	"github.com/windmilleng/tilt/internal/hud/proto"
	"github.com/windmilleng/tilt/internal/network"
	"google.golang.org/grpc"
)

var _ proto.HudServer = (*ServerAdapter)(nil)

func NewServer(ctx context.Context) (*ServerAdapter, error) {
	socketPath, err := proto.LocateSocket()
	if err != nil {
		return nil, err
	}

	l, err := network.UnixListen(socketPath)
	if err != nil {
		return nil, err
	}

	grpcServer := grpc.NewServer()

	a := &ServerAdapter{
		readyCh:        make(chan ReadyEvent),
		winchCh:        make(chan interface{}),
		streamClosedCh: make(chan error),
		serverClosed:   make(chan interface{}, 1),
		server:         grpcServer,
		ctx:            ctx,
	}

	proto.RegisterHudServer(grpcServer, a)

	// TODO(dbentley): deal with error
	go func() {
		err := grpcServer.Serve(l)
		if err != nil {
			log.Printf("hud server error: %v", err)
		}
	}()

	return a, nil
}

type ServerAdapter struct {
	readyCh        chan ReadyEvent
	winchCh        chan interface{}
	streamClosedCh chan error
	server         *grpc.Server
	ctx            context.Context
	serverClosed   chan interface{}
}

type ReadyEvent struct {
	ttyPath string
	ctx     context.Context
}

func (a *ServerAdapter) Close() {
	select {
	case <-a.serverClosed:
	default:
		close(a.serverClosed)
	}
	a.server.GracefulStop()
}

func (a *ServerAdapter) ConnectHud(stream proto.Hud_ConnectHudServer) error {
	streamContext := stream.Context()

	msg, err := stream.Recv()
	if err != nil {
		return err
	}

	// Expect first message to be a connect request
	connectMsg := msg.GetConnect()
	if connectMsg == nil {
		return fmt.Errorf("expected a connect msg; got %T %v", msg, msg)
	}

	ready := ReadyEvent{
		ttyPath: connectMsg.TtyPath,
		ctx:     streamContext,
	}
	a.readyCh <- ready

	go func() {
		for {
			_, err := stream.Recv() // assume it's a window change message
			if err != nil {
				a.streamClosedCh <- err
				return
			}
			a.winchCh <- nil
		}
	}()

	select {
	case <-streamContext.Done():
	case <-a.ctx.Done():
	case <-a.serverClosed:
	}

	return nil
}

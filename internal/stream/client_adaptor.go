package stream

import (
	"context"
	"io"

	"google.golang.org/grpc"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/network"
	"github.com/windmilleng/tilt/internal/stream/proto"
)

type MessageOrError struct {
	Err     error
	Message string
}

type StreamClient interface {
	Connect(ctx context.Context) (<-chan MessageOrError, error)
	Close() error
}

type streamClient struct {
	del proto.Stream_ConnectClient
}

func NewStreamClient() StreamClient {
	return &streamClient{}
}

func (s *streamClient) Close() error {
	if s.del != nil {
		return s.del.CloseSend()
	}
	return nil
}

func (s *streamClient) Connect(ctx context.Context) (<-chan MessageOrError, error) {
	socketPath, err := locateSocket()
	if err != nil {
		return nil, errors.Wrap(err, "error finding socket to connect to stream server")
	}

	conn, err := grpc.Dial(
		socketPath,
		grpc.WithInsecure(),
		grpc.WithDialer(network.UnixDial),
	)
	if err != nil {
		return nil, err
	}

	s.del, err = proto.NewStreamClient(conn).Connect(ctx, &proto.ConnectRequest{})
	if err != nil {
		return nil, errors.Wrap(err, "error connecting to stream server")
	}

	ch := make(chan MessageOrError)
	go func() {
		for {
			msg, err := s.del.Recv()
			if err != nil {
				if err != io.EOF {
					ch <- MessageOrError{Err: err}
				}
				close(ch)
				return
			}
			ch <- MessageOrError{Message: msg.S}
		}
	}()

	return ch, nil
}

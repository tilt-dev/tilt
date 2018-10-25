package stream

import (
	"context"
	"log"
	"sync"

	"google.golang.org/grpc"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/network"
	"github.com/windmilleng/tilt/internal/stream/proto"
)

type StreamServer interface {
	Send(s string)
	Close()
}

type streamServer struct {
	backlog     []string
	subscribers []chan string
	mu          sync.Mutex
	grpcServer  *grpc.Server
}

var _ StreamServer = &streamServer{}
var _ proto.StreamServer = &streamServer{}

func (s *streamServer) subscribe(ch chan string) {
	s.mu.Lock()
	s.subscribers = append(s.subscribers, ch)
	s.mu.Unlock()
}

func (s *streamServer) unsubscribe(ch chan string) {
	s.mu.Lock()
	var newSubscribers []chan string
	for _, s := range s.subscribers {
		if s != ch {
			newSubscribers = append(newSubscribers, s)
		}
	}
	s.mu.Unlock()
}

func (s *streamServer) Connect(req *proto.ConnectRequest, stream proto.Stream_ConnectServer) error {
	ch := make(chan string)
	s.subscribe(ch)
	defer s.unsubscribe(ch)

	s.mu.Lock()
	backlog := append([]string{}, s.backlog...)
	s.mu.Unlock()

	for _, msg := range backlog {
		err := stream.Send(&proto.StreamMessage{S: msg})
		if err != nil {
			return errors.Wrap(err, "error sending")
		}
	}

	for {
		select {
		case <-stream.Context().Done():
			err := stream.Context().Err()
			if err == context.Canceled {
				return nil
			} else {
				return err
			}
		case m, ok := <-ch:
			if !ok {
				return nil
			}
			err := stream.Send(&proto.StreamMessage{S: m})
			if err != nil {
				return errors.Wrap(err, "error sending")
			}
		}
	}
}

func (s *streamServer) Send(msg string) {
	s.mu.Lock()
	subs := append([]chan string{}, s.subscribers...)
	s.backlog = append(s.backlog, msg)
	s.mu.Unlock()

	for _, sub := range subs {
		sub <- msg
	}
}

func (s *streamServer) Close() {
	s.mu.Lock()
	subs := append([]chan string{}, s.subscribers...)
	s.subscribers = nil
	s.mu.Unlock()

	for _, sub := range subs {
		close(sub)
	}

	s.grpcServer.GracefulStop()
}

func NewServer(ctx context.Context) (StreamServer, error) {
	socketPath, err := locateSocket()
	if err != nil {
		return nil, errors.Wrap(err, "error opening stream server socket")
	}

	l, err := network.UnixListen(socketPath)
	if err != nil {
		return nil, errors.Wrap(err, "error opening stream server socket")
	}

	s := &streamServer{}

	s.grpcServer = grpc.NewServer()

	proto.RegisterStreamServer(s.grpcServer, s)

	go func() {
		err := s.grpcServer.Serve(l)
		if err != nil {
			log.Printf("stream server error: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		s.Close()
	}()

	return s, nil
}

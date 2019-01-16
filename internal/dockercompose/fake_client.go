package dockercompose

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/windmilleng/tilt/internal/model"
)

type FakeDCClient struct {
	t   *testing.T
	ctx context.Context

	RunLogOutput map[model.TargetName]<-chan string
	eventJson    chan string
}

func NewFakeDockerComposeClient(t *testing.T, ctx context.Context) *FakeDCClient {
	return &FakeDCClient{
		t:            t,
		ctx:          ctx,
		eventJson:    make(chan string, 100),
		RunLogOutput: make(map[model.TargetName]<-chan string),
	}
}

func (c *FakeDCClient) Up(ctx context.Context, pathToConfig string, serviceName model.TargetName, stdout, stderr io.Writer) error {
	return nil
}

func (c *FakeDCClient) Down(ctx context.Context, pathToConfig string, stdout, stderr io.Writer) error {
	return nil
}

func (c *FakeDCClient) StreamLogs(ctx context.Context, pathToConfig string, serviceName model.TargetName) (io.ReadCloser, error) {
	output := c.RunLogOutput[serviceName]
	reader, writer := io.Pipe()
	go func() {
		done := false
		for !done {
			select {
			case <-ctx.Done():
				done = true
			case s, ok := <-output:
				if !ok {
					done = true
				} else {
					_, _ = writer.Write([]byte(s))
				}
			}
		}
		_ = writer.Close()
	}()
	return reader, nil
}

func (c *FakeDCClient) StreamEvents(ctx context.Context, pathToConfig string) (<-chan string, error) {
	events := make(chan string, 10)
	go func() {
		for {
			select {
			case event := <-c.eventJson:
				select {
				case events <- event: // send event to channel (unless it's full)
				default:
					c.t.Fatalf("no room on events channel to send event: '%s'. Something "+
						"is wrong (or you need to increase the buffer).", event)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return events, nil
}

func (c *FakeDCClient) SendEvent(evt Event) error {
	j, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	c.eventJson <- string(j)
	return nil
}

func (c *FakeDCClient) Config(ctx context.Context, pathToConfig string) (string, error) {
	return "", nil
}

func (c *FakeDCClient) Services(ctx context.Context, pathToConfig string) (string, error) {
	return "", nil
}

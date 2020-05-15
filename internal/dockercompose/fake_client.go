package dockercompose

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/pkg/model"
)

type FakeDCClient struct {
	t   *testing.T
	ctx context.Context

	RunLogOutput      map[model.TargetName]<-chan string
	ContainerIdOutput container.ID
	eventJson         chan string
	ConfigOutput      string
	ServicesOutput    string

	UpCalls   []UpCall
	DownError error
}

// Represents a single call to Up
type UpCall struct {
	PathToConfig []string
	ServiceName  model.TargetName
	ShouldBuild  bool
}

func NewFakeDockerComposeClient(t *testing.T, ctx context.Context) *FakeDCClient {
	return &FakeDCClient{
		t:            t,
		ctx:          ctx,
		eventJson:    make(chan string, 100),
		RunLogOutput: make(map[model.TargetName]<-chan string),
	}
}

func (c *FakeDCClient) Up(ctx context.Context, configPaths []string, serviceName model.TargetName,
	shouldBuild bool, stdout, stderr io.Writer) error {
	c.UpCalls = append(c.UpCalls, UpCall{configPaths, serviceName, shouldBuild})
	return nil
}

func (c *FakeDCClient) Down(ctx context.Context, configPaths []string, stdout, stderr io.Writer) error {
	if c.DownError != nil {
		err := c.DownError
		c.DownError = err
		return err
	}
	return nil
}

func (c *FakeDCClient) StreamLogs(ctx context.Context, configPaths []string, serviceName model.TargetName) (io.ReadCloser, error) {
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

func (c *FakeDCClient) StreamEvents(ctx context.Context, configPaths []string) (<-chan string, error) {
	events := make(chan string, 10)
	go func() {
		for {
			select {
			case event := <-c.eventJson:
				select {
				case events <- event: // send event to channel (unless it's full)
				default:
					panic(fmt.Sprintf("no room on events channel to send event: '%s'. Something "+
						"is wrong (or you need to increase the buffer).", event))
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

func (c *FakeDCClient) Config(ctx context.Context, configPaths []string) (string, error) {
	return c.ConfigOutput, nil
}

func (c *FakeDCClient) Services(ctx context.Context, configPaths []string) (string, error) {
	return c.ServicesOutput, nil
}

func (c *FakeDCClient) ContainerID(ctx context.Context, configPaths []string, serviceName model.TargetName) (container.ID, error) {
	return c.ContainerIdOutput, nil
}

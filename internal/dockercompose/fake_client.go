package dockercompose

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"testing"
)

type FakeDCClient struct {
	t *testing.T

	logOutput string
	eventJson chan string
}

func NewFakeDockerComposeClient(t *testing.T) *FakeDCClient {
	return &FakeDCClient{
		t:         t,
		eventJson: make(chan string, 100),
	}
}

func (c *FakeDCClient) Up(ctx context.Context, pathToConfig, serviceName string, stdout, stderr io.Writer) error {
	return nil
}

func (c *FakeDCClient) Down(ctx context.Context, pathToConfig string, stdout, stderr io.Writer) error {
	return nil
}

func (c *FakeDCClient) StreamLogs(ctx context.Context, pathToConfig, serviceName string) (io.ReadCloser, error) {
	return ioutil.NopCloser(bytes.NewReader([]byte(c.logOutput))), nil
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

func (c *FakeDCClient) SetLogOutput(output string) {
	c.logOutput = output
}

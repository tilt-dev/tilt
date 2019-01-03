package dockercompose

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
)

type FakeDCClient struct {
	logOutput string

	eventJson chan string
}

// TODO(dmiller) make this configurable for testing
func NewFakeDockerComposeClient() *FakeDCClient {
	return &FakeDCClient{
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
	events := make(chan string, 100)
	go func() {
		for {
			select {
			case event := <-c.eventJson:
				events <- event
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

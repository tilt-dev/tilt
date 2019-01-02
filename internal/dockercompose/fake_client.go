package dockercompose

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
)

type FakeDCClient struct {
	// Log
	logOutput string
}

// TODO(dmiller) make this configurable for testing
func NewFakeDockerComposeClient() DockerComposeClient {
	return &FakeDCClient{}
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
	return nil, nil
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

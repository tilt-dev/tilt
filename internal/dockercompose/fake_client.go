package dockercompose

import (
	"context"
	"io"
)

type fakeDCClient struct{}

// TODO(dmiller) make this configurable for testing
func NewFakeDockerComposeClient() DockerComposeClient {
	return &fakeDCClient{}
}

func (c *fakeDCClient) Up(ctx context.Context, pathToConfig, serviceName string, stdout, stderr io.Writer) error {
	return nil
}

func (c *fakeDCClient) Down(ctx context.Context, pathToConfig string, stdout, stderr io.Writer) error {
	return nil
}

func (c *fakeDCClient) Logs(ctx context.Context, pathToConfig, serviceName string) (io.ReadCloser, error) {
	return nil, nil
}

func (c *fakeDCClient) Events(ctx context.Context, pathToConfig string) (<-chan string, error) {
	return nil, nil
}

func (c *fakeDCClient) Config(ctx context.Context, pathToConfig string) (string, error) {
	return "", nil
}

func (c *fakeDCClient) Services(ctx context.Context, pathToConfig string) (string, error) {
	return "", nil
}

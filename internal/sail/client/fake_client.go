package client

import (
	"context"

	"github.com/windmilleng/tilt/internal/store"
)

type FakeSailClient struct {
	ConnectCalls int
}

var _ SailClient = &sailClient{}

func NewFakeSailClient() *FakeSailClient {
	return &FakeSailClient{}
}

func (c *FakeSailClient) OnChange(ctx context.Context, st store.RStore) {}
func (c *FakeSailClient) Teardown(ctx context.Context)                  {}

func (c *FakeSailClient) NewRoom(ctx context.Context, st store.RStore) error {
	c.ConnectCalls += 1
	return nil
}

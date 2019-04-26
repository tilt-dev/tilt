package client

import (
	"context"

	"github.com/windmilleng/tilt/internal/store"
)

type FakeSailClient struct {
	MaybeBroadcastCalls int
	ConnectCalls        int
}

var _ SailClient = &sailClient{}

func NewFakeSailClient() *FakeSailClient {
	return &FakeSailClient{}
}

func (c *FakeSailClient) MaybeBroadcast(st store.RStore) {
	c.MaybeBroadcastCalls += 1
}

func (c *FakeSailClient) Connect(st store.RStore) error {
	c.ConnectCalls += 1
	return nil
}

func (c *FakeSailClient) Teardown(ctx context.Context) {}

package client

import (
	"context"
)

type fakeClient struct{}

func (f *fakeClient) SendAlert(ctx context.Context, alert Alert) (AlertID, error) {
	return "aaaaaa", nil
}

func ProvideFakeClient() Client {
	return &fakeClient{}
}

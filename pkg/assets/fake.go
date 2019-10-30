package assets

import (
	"context"
	"net/http"
)

type fakeServer struct {
}

func NewFakeServer() fakeServer {
	return fakeServer{}
}

func (fakeServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {}
func (fakeServer) Serve(ctx context.Context) error {
	return nil
}
func (fakeServer) TearDown(ctx context.Context) {}

var _ Server = fakeServer{}

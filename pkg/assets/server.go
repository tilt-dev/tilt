package assets

import (
	"context"
	"net/http"
	"net/url"
)

// The directory where the package.json where our JS source code lives.
type PackageDir string

func (d PackageDir) String() string {
	return string(d)
}

type Server interface {
	http.Handler
	Serve(ctx context.Context) error
	TearDown(ctx context.Context)
}

func NewFakeServer() Server {
	loc, err := url.Parse("https://fake.tilt.dev")
	if err != nil {
		panic(err)
	}
	return prodServer{baseUrl: loc}
}

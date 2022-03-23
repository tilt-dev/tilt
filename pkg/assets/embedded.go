package assets

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/tilt-dev/tilt/pkg/logger"
)

//go:embed build
var build embed.FS

type embeddedServer struct {
	http.Handler
}

func NewEmbeddedServer() (Server, error) {
	var assets fs.FS
	index, err := build.ReadFile("build/index.html")
	if err == nil {
		assets, err = fs.Sub(build, "build")
	}

	if err != nil {
		return embeddedServer{}, fmt.Errorf("embedded assets unavailable: %w", err)
	}

	return embeddedServer{
		Handler: handleRoutingURLs(http.FileServer(http.FS(assets)), index),
	}, nil
}

func (s embeddedServer) Serve(ctx context.Context) error {
	logger.Get(ctx).Verbosef("Serving embedded Tilt production web assets")
	<-ctx.Done()
	return nil
}

func (s embeddedServer) TearDown(ctx context.Context) {
}

func handleRoutingURLs(handler http.Handler, index []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.URL.Path, "/r/") {
			w.Header().Set("Cache-Control", "no-store, max-age=0")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Content-Length", strconv.Itoa(len(index)))
			w.WriteHeader(200)
			_, _ = w.Write(index)
		} else {
			handler.ServeHTTP(w, req)
		}
	}
}

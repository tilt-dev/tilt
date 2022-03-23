package assets

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"strconv"

	"github.com/tilt-dev/tilt/pkg/logger"
)

//go:embed build
var build embed.FS

type embeddedServer struct {
	http.Handler
}

func GetEmbeddedServer() (Server, bool) {
	var assets fs.FS
	index, err := build.ReadFile("build/index.html")
	if err == nil {
		assets, err = fs.Sub(build, "build")
	}

	if err != nil {
		return embeddedServer{}, false
	}

	return embeddedServer{Handler: serveAssets(assets, index)}, true
}

func (s embeddedServer) Serve(ctx context.Context) error {
	logger.Get(ctx).Verbosef("Serving embedded Tilt production web assets")
	<-ctx.Done()
	return nil
}

func (s embeddedServer) TearDown(ctx context.Context) {
}

func serveAssets(assets fs.FS, index []byte) http.HandlerFunc {
	handler := http.FileServer(http.FS(assets))
	return func(w http.ResponseWriter, req *http.Request) {
		w = cacheAssets(w, req.URL.Path, req.Method)
		if isAssetPath(req.URL.Path) {
			handler.ServeHTTP(w, req)
		} else {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Content-Length", strconv.Itoa(len(index)))
			w.WriteHeader(200)
			_, _ = w.Write(index)
		}
	}
}

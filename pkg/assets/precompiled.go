package assets

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/windmilleng/tilt/internal/ospath"
	"github.com/windmilleng/tilt/pkg/logger"
)

type precompiledServer struct {
	packageDir PackageDir
}

func NewPrecompiledServer(packageDir PackageDir) Server {
	return precompiledServer{
		packageDir: packageDir,
	}
}

func (s precompiledServer) TearDown(ctx context.Context) {
}

func (s precompiledServer) buildPath() string {
	return path.Join(s.packageDir.String(), "build")
}

// This doesn't actually do any setup right now.
func (s precompiledServer) Serve(ctx context.Context) error {
	logger.Get(ctx).Verbosef("Serving Tilt production precompiled assets from %s", s.buildPath())
	<-ctx.Done()
	return nil
}

func (s precompiledServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	origPath := req.URL.Path
	if !strings.HasPrefix(origPath, "/static/") {
		// redirect everything else to the main entry point.
		origPath = "index.html"
	}

	contentPath := path.Join(s.buildPath(), origPath)
	if !ospath.IsChild(s.buildPath(), contentPath) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(fmt.Sprintf("Access Forbidden: %s", contentPath)))
		return

	}

	f, err := os.Open(contentPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	defer func() {
		_ = f.Close()
	}()

	w.Header().Add("Content-Type", mime.TypeByExtension(filepath.Ext(contentPath)))
	_, _ = io.Copy(w, f)
}

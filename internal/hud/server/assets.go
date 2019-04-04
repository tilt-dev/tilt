package server

import (
	"bytes"
	"context"
	"fmt"
	"go/build"
	"io"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/network"
	"github.com/windmilleng/tilt/internal/ospath"
)

type AssetServer interface {
	http.Handler
	Serve(ctx context.Context) error
	Teardown(ctx context.Context)
}

type devAssetServer struct {
	http.Handler
	port     model.WebDevPort
	mu       sync.Mutex
	cmd      *exec.Cmd
	disposed bool
}

func (s *devAssetServer) Teardown(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cmd := s.cmd
	if cmd != nil && cmd.Process != nil {
		// Kill the entire process group.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	s.disposed = true
}

func (s *devAssetServer) start(ctx context.Context, stdout, stderr io.Writer) (*exec.Cmd, error) {
	myPkg, err := build.Default.Import("github.com/windmilleng/tilt/internal/hud/server", ".", build.FindOnly)
	if err != nil {
		return nil, errors.Wrap(err, "Could not locate Tilt source code")
	}

	myDir := myPkg.Dir
	assetDir := filepath.Join(myDir, "..", "..", "..", "web")

	logger.Get(ctx).Infof("Installing Tilt NodeJS dependencies…")
	cmd := exec.CommandContext(ctx, "yarn", "install")
	cmd.Dir = assetDir
	cmd.Stdout = logger.Get(ctx).Writer(logger.DebugLvl)
	cmd.Stderr = logger.Get(ctx).Writer(logger.DebugLvl)
	err = cmd.Run()
	if err != nil {
		return nil, errors.Wrap(err, "Installing Tilt webpack deps")
	}

	logger.Get(ctx).Infof("Starting Tilt webpack server…")
	cmd = exec.CommandContext(ctx, "yarn", "run", "start")
	cmd.Dir = assetDir
	cmd.Env = append(os.Environ(), "BROWSER=none", fmt.Sprintf("PORT=%d", s.port))

	// yarn will spawn the dev server as a subproces, so set
	// a process group id so we can murder them all.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.disposed {
		return nil, nil
	}

	err = cmd.Start()
	if err != nil {
		return nil, errors.Wrap(err, "Starting dev web server")
	}

	s.cmd = cmd
	return cmd, nil
}

func (s *devAssetServer) Serve(ctx context.Context) error {
	if !network.IsPortFree(int(s.port)) {
		return fmt.Errorf("Cannot start Tilt dev webpack server. "+
			"Another process is already running on port %d. "+
			"Use --webdev-port to set a custom port", s.port)
	}

	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	cmd, err := s.start(ctx, stdout, stderr)
	if cmd == nil || err != nil {
		return err
	}

	err = cmd.Wait()
	if ctx.Err() != nil {
		// Process was killed
		return nil
	}

	if err != nil {
		exitErr, isExit := err.(*exec.ExitError)
		if isExit {
			return errors.Wrapf(err, "Running dev web server. Stderr: %s", string(exitErr.Stderr))
		}
		return errors.Wrap(err, "Running dev web server")
	}
	return fmt.Errorf("Tilt dev server stopped unexpectedly\nStdout:\n%s\nStderr:\n%s\n",
		stdout.String(), stderr.String())
}

type prodAssetServer struct {
	url *url.URL
}

func (s prodAssetServer) Teardown(ctx context.Context) {
}

// This doesn't actually do any setup right now.
func (s prodAssetServer) Serve(ctx context.Context) error {
	logger.Get(ctx).Verbosef("Serving Tilt production web assets from %s", s.url)
	<-ctx.Done()
	return nil
}

// NOTE(nick): The reverse proxy in httputil makes the storage server 500 and I have no idea
// why. But this only needs a very limited GET interface without query params,
// so just make the request by hand.
func (s prodAssetServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	outurl := *s.url
	origPath := req.URL.Path
	if !strings.HasPrefix(origPath, "/static/") {
		// redirect everything to the main entry point.
		origPath = "index.html"
	}

	outurl.Path = path.Join(outurl.Path, origPath)
	outreq, err := http.NewRequest("GET", outurl.String(), bytes.NewBuffer(nil))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	copyHeader(outreq.Header, req.Header)
	outres, err := http.DefaultClient.Do(outreq)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	defer func() {
		_ = outres.Body.Close()
	}()

	copyHeader(w.Header(), outres.Header)
	w.WriteHeader(outres.StatusCode)
	_, _ = io.Copy(w, outres.Body)
}

type precompiledAssetServer struct {
	path string
}

func (s precompiledAssetServer) Teardown(ctx context.Context) {
}

// This doesn't actually do any setup right now.
func (s precompiledAssetServer) Serve(ctx context.Context) error {
	logger.Get(ctx).Verbosef("Serving Tilt production precompiled assets from %s", s.path)
	<-ctx.Done()
	return nil
}

func (s precompiledAssetServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	origPath := req.URL.Path
	if !strings.HasPrefix(origPath, "/static/") {
		// redirect everything to the main entry point.
		origPath = "index.html"
	}

	contentPath := path.Join(s.path, origPath)
	if !ospath.IsChild(s.path, contentPath) {
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

func NewFakeAssetServer() AssetServer {
	loc, err := url.Parse("https://fake.tilt.dev")
	if err != nil {
		panic(err)
	}
	return prodAssetServer{url: loc}
}

func ProvideAssetServer(ctx context.Context, webMode model.WebMode, webVersion model.WebVersion, devPort model.WebDevPort) (AssetServer, error) {
	if webMode == model.LocalWebMode {
		loc, err := url.Parse(fmt.Sprintf("http://localhost:%d", devPort))
		if err != nil {
			return nil, errors.Wrap(err, "ProvideAssetServer")
		}

		return &devAssetServer{
			Handler: httputil.NewSingleHostReverseProxy(loc),
			port:    devPort,
		}, nil
	}

	if webMode == model.PrecompiledWebMode {
		pkg, err := build.Default.Import("github.com/windmilleng/tilt/internal/hud/server", ".", build.FindOnly)
		if err != nil {
			return nil, fmt.Errorf("Precompiled JS not found on disk: %v", err)
		}

		buildDir := filepath.Join(pkg.Dir, "../../../web/build")
		return precompiledAssetServer{
			path: buildDir,
		}, nil
	}

	if webMode == model.ProdWebMode {
		loc, err := url.Parse(fmt.Sprintf("https://storage.googleapis.com/tilt-static-assets/%s", webVersion))
		if err != nil {
			return nil, errors.Wrap(err, "ProvideAssetServer")
		}
		return prodAssetServer{
			url: loc,
		}, nil
	}

	return nil, model.UnrecognizedWebModeError(string(webMode))
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

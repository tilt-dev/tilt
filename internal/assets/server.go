package assets

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/network"
	"github.com/windmilleng/tilt/internal/ospath"
	"github.com/windmilleng/tilt/web"
)

const prodAssetBucket = "https://storage.googleapis.com/tilt-static-assets/"
const WebVersionKey = "web_version"

const errorBodyStyle = `
  font-family: Inconsolata, monospace;
  background-color: #002b36;
  color: #ffffff;
  font-size: 20px;
  line-height: 1.5;
  margin: 0;
`
const errorDivStyle = `
  width: 100%;
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
`

var versionRe = regexp.MustCompile(`/(v\d+\.\d+\.\d+)/.*`) // matches `/vA.B.C/...`

type Server interface {
	http.Handler
	Serve(ctx context.Context) error
	TearDown(ctx context.Context)
}

type devServer struct {
	http.Handler
	port     model.WebDevPort
	mu       sync.Mutex
	cmd      *exec.Cmd
	disposed bool
}

func (s *devServer) TearDown(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cmd := s.cmd
	killProcessGroup(cmd)
	s.disposed = true
}

func (s *devServer) start(ctx context.Context, stdout, stderr io.Writer) (*exec.Cmd, error) {
	assetDir, err := web.StaticPath()
	if err != nil {
		return nil, err
	}

	logger.Get(ctx).Infof("Installing Tilt NodeJS dependencies…")
	cmd := exec.CommandContext(ctx, "yarn", "install")
	cmd.Dir = assetDir
	stdoutString := &strings.Builder{}
	stderrString := &strings.Builder{}
	stdoutWriter := io.MultiWriter(stdoutString, logger.Get(ctx).Writer(logger.DebugLvl))
	stderrWriter := io.MultiWriter(stderrString, logger.Get(ctx).Writer(logger.DebugLvl))
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter
	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("Error installing Tilt webpack deps:\nstdout:\n%s\nstderr:\n%s\nerror: %s", stdoutString.String(), stderrString.String(), err)
	}

	logger.Get(ctx).Infof("Starting Tilt webpack server…")
	cmd = exec.CommandContext(ctx, "yarn", "run", "start")
	cmd.Dir = assetDir
	cmd.Env = append(os.Environ(), "BROWSER=none", fmt.Sprintf("PORT=%d", s.port))

	attrs := &syscall.SysProcAttr{}

	// yarn will spawn the dev server as a subprocess, so set
	// a process group id so we can murder them all.
	setOptNewProcessGroup(attrs)

	cmd.SysProcAttr = attrs

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

func (s *devServer) Serve(ctx context.Context) error {
	// webpack binds to 0.0.0.0
	err := network.IsBindAddrFree(network.AllHostsBindAddr(int(s.port)))
	if err != nil {
		return errors.Wrapf(err, "Cannot start Tilt dev webpack server. "+
			"Maybe another process is already running on port %d? "+
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

type prodServer struct {
	baseUrl        *url.URL
	defaultVersion model.WebVersion
}

func newProdServer(baseUrl string, version model.WebVersion) (prodServer, error) {
	loc, err := url.Parse(baseUrl)
	if err != nil {
		return prodServer{}, errors.Wrap(err, "ProvideAssetServer")
	}
	return prodServer{
		baseUrl:        loc,
		defaultVersion: version,
	}, nil
}
func (s prodServer) TearDown(ctx context.Context) {
}

// This doesn't actually do any setup right now.
func (s prodServer) Serve(ctx context.Context) error {
	logger.Get(ctx).Verbosef("Serving Tilt production web assets from %s with default version %s",
		s.baseUrl, s.defaultVersion)
	<-ctx.Done()
	return nil
}

// NOTE(nick): The reverse proxy in httputil makes the storage server 500 and I have no idea
// why. But this only needs a very limited GET interface without query params,
// so just make the request by hand.
func (s prodServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	outurl, version := s.urlAndVersionForReq(req)
	outreq, err := http.NewRequest("GET", outurl.String(), bytes.NewBuffer(nil))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	copyHeader(outreq.Header, req.Header)
	outres, err := http.DefaultClient.Do(outreq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer func() {
		_ = outres.Body.Close()
	}()

	// In case we change the length of the response below, we don't want the browser to be mad
	outres.Header.Del("Content-Length")
	copyHeader(w.Header(), outres.Header)

	w.WriteHeader(outres.StatusCode)
	resBody, _ := ioutil.ReadAll(outres.Body)
	resBodyWithVersion := s.injectVersion(resBody, version)
	_, _ = w.Write(resBodyWithVersion)
}

func (s prodServer) urlAndVersionForReq(req *http.Request) (url.URL, string) {
	u := *s.baseUrl
	origPath := req.URL.Path

	if matches := versionRe.FindStringSubmatch(origPath); len(matches) > 1 {
		// If url contains a version prefix, don't attach another version
		u.Path = path.Join(u.Path, origPath)
		return u, matches[1]
	}

	if !(strings.HasPrefix(origPath, "/static/")) {
		// redirect everything else to the main entry point.
		origPath = "index.html"
	}

	version := req.URL.Query().Get(WebVersionKey)
	if version == "" {
		version = string(s.defaultVersion)
	}

	u.Path = path.Join(u.Path, version, origPath)
	return u, version
}

// injectVersion updates all links to "/static/..." to instead point to "/vA.B.C/static/..."
// We do this b/c asset index.html's may contain links to "/static/..." that don't specify the
// version prefix, but leave it up to the asset server to resolve. Now that the asset server
// may serve multiple versions at once, we need to specify.
func (s prodServer) injectVersion(html []byte, version string) []byte {
	newPrefix := fmt.Sprintf("/%s/static/", version)
	return bytes.ReplaceAll(html, []byte("/static/"), []byte(newPrefix))
}

type precompiledServer struct {
	path string
}

func (s precompiledServer) TearDown(ctx context.Context) {
}

// This doesn't actually do any setup right now.
func (s precompiledServer) Serve(ctx context.Context) error {
	logger.Get(ctx).Verbosef("Serving Tilt production precompiled assets from %s", s.path)
	<-ctx.Done()
	return nil
}

func (s precompiledServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	origPath := req.URL.Path
	if !strings.HasPrefix(origPath, "/static/") {
		// redirect everything else to the main entry point.
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

func NewFakeServer() Server {
	loc, err := url.Parse("https://fake.tilt.dev")
	if err != nil {
		panic(err)
	}
	return prodServer{baseUrl: loc}
}

func ProvideAssetServer(ctx context.Context, webMode model.WebMode, webVersion model.WebVersion, devPort model.WebDevPort) (Server, error) {
	if webMode == model.LocalWebMode {
		loc, err := url.Parse(fmt.Sprintf("http://localhost:%d", devPort))
		if err != nil {
			return nil, errors.Wrap(err, "ProvideAssetServer")
		}
		handler := httputil.NewSingleHostReverseProxy(loc)
		handler.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, e error) {
			extra := ""
			extraHead := ""
			if strings.Contains(e.Error(), "connection refused") {
				extra = `"connection refused" expected on startup with --web-mode=local. Refreshing in a few seconds.`
				extraHead = `<meta http-equiv="refresh" content="2">`
			}
			response := fmt.Sprintf(`
<html>
  <head>%s</head>
  <body style="%s">
    <div style="%s">
      Error talking to asset server:<pre>%s</pre>
      <br>%s
    </div>
  </body>
</html>`, extraHead, errorBodyStyle, errorDivStyle, e.Error(), extra)
			_, _ = writer.Write([]byte(response))
		}

		return &devServer{
			Handler: handler,
			port:    devPort,
		}, nil
	}

	if webMode == model.PrecompiledWebMode {
		assetDir, err := web.StaticPath()
		if err != nil {
			return nil, fmt.Errorf("Precompiled JS: %v", err)
		}

		buildDir := filepath.Join(assetDir, "build")
		return precompiledServer{
			path: buildDir,
		}, nil
	}

	if webMode == model.ProdWebMode {
		return newProdServer(prodAssetBucket, webVersion)
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

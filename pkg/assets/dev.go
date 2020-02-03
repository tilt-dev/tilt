package assets

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
	"github.com/windmilleng/tilt/pkg/procutil"
)

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

type devServer struct {
	http.Handler
	packageDir PackageDir
	port       model.WebDevPort

	mu       sync.Mutex
	cmd      *exec.Cmd
	disposed bool
}

func NewDevServer(packageDir PackageDir, devPort model.WebDevPort) (*devServer, error) {
	loc, err := url.Parse(fmt.Sprintf("http://localhost:%d", devPort))
	if err != nil {
		return nil, errors.Wrap(err, "NewDevServer")
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
		Handler:    handler,
		packageDir: packageDir,
		port:       devPort,
	}, nil
}

func (s *devServer) TearDown(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cmd := s.cmd
	procutil.KillProcessGroup(cmd)
	s.disposed = true
}

func (s *devServer) start(ctx context.Context, stdout, stderr io.Writer) (*exec.Cmd, error) {
	logger.Get(ctx).Infof("Installing Tilt NodeJS dependencies…")
	cmd := exec.CommandContext(ctx, "yarn", "install")
	cmd.Dir = s.packageDir.String()
	stdoutString := &strings.Builder{}
	stderrString := &strings.Builder{}
	stdoutWriter := io.MultiWriter(stdoutString, logger.Get(ctx).Writer(logger.DebugLvl))
	stderrWriter := io.MultiWriter(stderrString, logger.Get(ctx).Writer(logger.DebugLvl))
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("Error installing Tilt webpack deps:\nstdout:\n%s\nstderr:\n%s\nerror: %s", stdoutString.String(), stderrString.String(), err)
	}

	logger.Get(ctx).Infof("Starting Tilt webpack server…")
	cmd = exec.CommandContext(ctx, "yarn", "run", "start")
	cmd.Dir = s.packageDir.String()
	cmd.Env = append(os.Environ(), "BROWSER=none", fmt.Sprintf("PORT=%d", s.port))

	attrs := &syscall.SysProcAttr{}

	// yarn will spawn the dev server as a subprocess, so set
	// a process group id so we can murder them all.
	procutil.SetOptNewProcessGroup(attrs)

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
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", int(s.port)))
	if err != nil {
		return errors.Wrapf(err, "Cannot start Tilt dev webpack server. "+
			"Maybe another process is already running on port %d? "+
			"Use --webdev-port to set a custom port", s.port)
	}
	_ = l.Close()

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

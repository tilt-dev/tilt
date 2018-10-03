//+build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"go/build"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
)

var packageDir string

const namespaceFlag = "-n=tilt-integration"

func init() {
	pkg, err := build.Default.Import("github.com/windmilleng/tilt/integration", ".", build.FindOnly)
	if err != nil {
		panic(fmt.Sprintf("Could not find integration test source code: %v", err))
	}

	packageDir = pkg.Dir
}

type fixture struct {
	t      *testing.T
	ctx    context.Context
	cancel func()
	logs   *bytes.Buffer
	cmds   []*exec.Cmd
}

func newFixture(t *testing.T, dir string) *fixture {
	err := os.Chdir(filepath.Join(packageDir, dir))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	f := &fixture{
		t:      t,
		ctx:    ctx,
		cancel: cancel,
		logs:   bytes.NewBuffer(nil),
	}
	f.CreateNamespaceIfNecessary()
	f.ClearNamespace()
	return f
}

func (f *fixture) Curl(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", errors.Wrap(err, "Curl")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		f.t.Errorf("Error fetching %s: %s", url, resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "Curl")
	}
	return string(body), nil
}

func (f *fixture) CurlUntil(ctx context.Context, url string, expectedContents string) {
	for {
		actualContents, err := f.Curl(url)
		if err == nil && strings.Contains(actualContents, expectedContents) {
			return
		}

		select {
		case <-ctx.Done():
			f.t.Fatalf("Timed out waiting for server %q to return contents %s. "+
				"Current error:\n%v\nCurrent contents:\n%s\n",
				url, expectedContents, err, actualContents)
		case <-time.After(200 * time.Millisecond):
		}
	}

}

func (f *fixture) tiltCmd(tiltArgs []string, outWriter io.Writer) *exec.Cmd {
	outWriter = io.MultiWriter(f.logs, outWriter)

	gopath := os.Getenv("GOPATH")
	args := []string{
		"run",
		filepath.Join(gopath, "src/github.com/windmilleng/tilt/cmd/tilt/main.go"),
	}
	args = append(args, tiltArgs...)
	cmd := exec.CommandContext(f.ctx, "go", args...)
	cmd.Stdout = outWriter
	cmd.Stderr = outWriter
	return cmd
}

func (f *fixture) TiltUp(name string) {
	out := bytes.NewBuffer(nil)
	cmd := f.tiltCmd([]string{"up", name, "--debug"}, out)
	err := cmd.Run()
	if err != nil {
		f.t.Fatalf("Failed to up service: %v. Logs:\n%s", err, out.String())
	}
}

func (f *fixture) ForwardPort(name string, portMap string) {
	outWriter := os.Stdout
	cmd := exec.CommandContext(f.ctx, "kubectl", "port-forward", namespaceFlag, name, portMap)
	cmd.Stdout = outWriter
	cmd.Stderr = outWriter
	err := cmd.Start()
	if err != nil {
		f.t.Fatal(err)
	}

	f.cmds = append(f.cmds, cmd)
	go func() {
		_ = cmd.Wait()
	}()
}

func (f *fixture) ClearResource(name string) {
	outWriter := bytes.NewBuffer(nil)
	cmd := exec.CommandContext(f.ctx, "kubectl", "delete", name, namespaceFlag, "--all")
	cmd.Stdout = outWriter
	cmd.Stderr = outWriter
	err := cmd.Run()
	if err != nil {
		f.t.Fatalf("Error deleting deployments: %v. Logs:\n%s", err, outWriter.String())
	}
}

func (f *fixture) CreateNamespaceIfNecessary() {
	outWriter := bytes.NewBuffer(nil)
	cmd := exec.CommandContext(f.ctx, "kubectl", "apply", "-f", "namespace.yaml")
	cmd.Stdout = outWriter
	cmd.Stderr = outWriter
	cmd.Dir = packageDir
	err := cmd.Run()
	if err != nil {
		f.t.Fatalf("Error creating namespace: %v. Logs:\n%s", err, outWriter.String())
	}
}

func (f *fixture) ClearNamespace() {
	f.ClearResource("deployments")
	f.ClearResource("services")
}

func (f *fixture) TearDown() {
	for _, cmd := range f.cmds {
		process := cmd.Process
		if process != nil {
			process.Kill()
		}
	}

	f.ClearNamespace()

	f.cancel()
}

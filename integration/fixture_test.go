// +build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/windmilleng/tilt/internal/testutils/bufsync"
)

var packageDir string
var imageTagPrefix string
var installed bool

const namespaceFlag = "-n=tilt-integration"

func init() {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic(fmt.Errorf("Could not locate path to Tilt integration tests"))
	}

	packageDir = filepath.Dir(file)
	imageTagPrefix = fmt.Sprintf("tilt-T-%x-", time.Now().Unix())
}

type fixture struct {
	t             *testing.T
	ctx           context.Context
	cancel        func()
	dir           string
	logs          *bufsync.ThreadSafeBuffer
	cmds          []*exec.Cmd
	originalFiles map[string]string
	tiltEnviron   map[string]string
	tearingDown   bool
}

func newFixture(t *testing.T, dir string) *fixture {
	dir = filepath.Join(packageDir, dir)
	err := os.Chdir(dir)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	f := &fixture{
		t:             t,
		ctx:           ctx,
		cancel:        cancel,
		dir:           dir,
		logs:          bufsync.NewThreadSafeBuffer(),
		originalFiles: make(map[string]string),
		tiltEnviron: map[string]string{
			"TILT_DISABLE_ANALYTICS": "true",
			"TILT_K8S_EVENTS":        "true",
		},
	}

	if !installed {
		// Install tilt on the first test run.
		f.installTilt()

		// Delete the namespace when the test starts,
		// to make sure nothing is left over from previous tests.
		f.deleteNamespace()

		installed = true
	}

	return f
}

func (f *fixture) installTilt() {
	cmd := exec.CommandContext(f.ctx, "go", "install", "github.com/windmilleng/tilt/cmd/tilt")
	err := cmd.Run()
	if err != nil {
		f.t.Fatalf("Building tilt: %v", err)
	}
}

func (f *fixture) deleteNamespace() {
	cmd := exec.CommandContext(f.ctx, "kubectl", "delete", "namespace", "tilt-integration", "--ignore-not-found")
	err := cmd.Run()
	if err != nil {
		f.t.Fatalf("Deleting namespace tilt-integration: %v", err)
	}

	// block until the namespace doesn't exist, since kubectl often returns and the namespace is still "terminating"
	// which causes the creation of objects in that namespace to fail
	var b []byte
	args := []string{"kubectl", "get", "namespace", "tilt-integration", "--ignore-not-found"}
	timeout := time.Now().Add(10 * time.Second)
	for time.Now().Before(timeout) {
		cmd := exec.CommandContext(f.ctx, args[0], args[1:]...)
		b, err = cmd.Output()
		if err != nil {
			f.t.Fatalf("Error: checking that deletion of the tilt-integration namespace has completed: %v", err)
		}
		if len(b) == 0 {
			return
		}
	}
	f.t.Fatalf("timed out waiting for tilt-integration deletion to complete. last output of %q: %q", args, string(b))
}

func (f *fixture) DumpLogs() {
	_, _ = os.Stdout.Write([]byte(f.logs.String()))
}

func (f *fixture) WaitUntil(ctx context.Context, msg string, fun func() (string, error), expectedContents string) {
	for {
		actualContents, err := fun()
		if err == nil && strings.Contains(actualContents, expectedContents) {
			return
		}

		select {
		case <-ctx.Done():
			f.KillProcs()
			f.t.Fatalf("Timed out waiting for expected result (%s)\n"+
				"Expected: %s\n"+
				"Actual: %s\n"+
				"Current error: %v\n",
				msg, expectedContents, actualContents, err)
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (f *fixture) tiltCmd(tiltArgs []string, outWriter io.Writer) *exec.Cmd {
	outWriter = io.MultiWriter(f.logs, outWriter)
	cmd := exec.CommandContext(f.ctx, "tilt", tiltArgs...)
	cmd.Stdout = outWriter
	cmd.Stderr = outWriter
	cmd.Env = append(os.Environ())
	for k, v := range f.tiltEnviron {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	return cmd
}

func (f *fixture) TiltUp(name string) {
	out := bytes.NewBuffer(nil)
	cmd := f.tiltCmd([]string{"up", name, "--watch=false", "--debug", "--hud=false", "--port=0", "--image-tag-prefix=" + imageTagPrefix}, out)
	err := cmd.Run()
	if err != nil {
		f.t.Fatalf("Failed to up service: %v. Logs:\n%s", err, out.String())
	}
}

func (f *fixture) runInBackground(cmd *exec.Cmd) {
	err := cmd.Start()
	if err != nil {
		f.t.Fatal(err)
	}

	f.cmds = append(f.cmds, cmd)
	go func() {
		err = cmd.Wait()
		if err != nil {
			fmt.Printf("error running command: %v\n", err)
			if ee, ok := err.(*exec.ExitError); ok {
				fmt.Printf("stderr: %q\n", ee.Stderr)
			}
		}
	}()
}

func (f *fixture) TiltWatch() {
	cmd := f.tiltCmd([]string{"up", "--debug", "--hud=false", "--port=0"}, os.Stdout)
	f.runInBackground(cmd)
}

func (f *fixture) TiltWatchExec() {
	cmd := f.tiltCmd([]string{"up", "--debug", "--hud=false", "--port=0", "--update-mode", "exec"}, os.Stdout)
	f.runInBackground(cmd)
}

func (f *fixture) ReplaceContents(fileBaseName, original, replacement string) {
	file := filepath.Join(f.dir, fileBaseName)
	contentsBytes, err := ioutil.ReadFile(file)
	if err != nil {
		f.t.Fatal(err)
	}

	contents := string(contentsBytes)
	_, hasStoredContents := f.originalFiles[file]
	if !hasStoredContents {
		f.originalFiles[file] = contents
	}

	newContents := strings.Replace(contents, original, replacement, -1)
	if newContents == contents {
		f.t.Fatalf("Could not find contents %q to replace in file %s: %s", original, fileBaseName, contents)
	}

	err = ioutil.WriteFile(file, []byte(newContents), os.FileMode(0777))
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) StartTearDown() {
	f.cancel()
	f.ctx = context.Background()
	f.tearingDown = true
}

func (f *fixture) KillProcs() {
	for _, cmd := range f.cmds {
		process := cmd.Process
		if process != nil {
			process.Kill()
		}
	}
}
func (f *fixture) TearDown() {
	f.StartTearDown()

	f.KillProcs()

	// This is a hack.
	//
	// Deleting a namespace is slow. Doing it on every test case makes
	// the tests more accurate. We believe that in this particular case,
	// the trade-off of speed over accuracy is worthwhile, so
	// we add this hack so that we can `tilt down` without deleting
	// the namespace.
	//
	// Each Tiltfile reads this environment variable, and skips loading the namespace
	// into Tilt, so that Tilt doesn't delete it.
	//
	// If users want to do the same thing in practice, it might be worth
	// adding better in-product hooks (e.g., `tilt down --preserve-namespace`),
	// or more scriptability in the Tiltfile.
	f.tiltEnviron["SKIP_NAMESPACE"] = "true"

	cmd := f.tiltCmd([]string{"down"}, os.Stdout)
	err := cmd.Run()
	if err != nil {
		f.t.Fatal(err)
	}

	for k, v := range f.originalFiles {
		_ = ioutil.WriteFile(k, []byte(v), os.FileMode(0777))
	}
}

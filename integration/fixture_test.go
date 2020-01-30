// +build integration

package integration

import (
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
var installed bool

const namespaceFlag = "-n=tilt-integration"

func init() {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic(fmt.Errorf("Could not locate path to Tilt integration tests"))
	}

	packageDir = filepath.Dir(file)
}

type fixture struct {
	t             *testing.T
	ctx           context.Context
	cancel        func()
	dir           string
	logs          *bufsync.ThreadSafeBuffer
	originalFiles map[string]string
	tilt          *TiltDriver
	activeTiltUp  *TiltUpResponse
	tearingDown   bool
	tiltArgs      []string
}

func newFixture(t *testing.T, dir string) *fixture {
	dir = filepath.Join(packageDir, dir)
	err := os.Chdir(dir)
	if err != nil {
		t.Fatal(err)
	}

	client := NewTiltDriver()
	client.Environ["TILT_DISABLE_ANALYTICS"] = "true"

	ctx, cancel := context.WithCancel(context.Background())
	f := &fixture{
		t:             t,
		ctx:           ctx,
		cancel:        cancel,
		dir:           dir,
		logs:          bufsync.NewThreadSafeBuffer(),
		originalFiles: make(map[string]string),
		tilt:          client,
	}

	if !installed {
		// Install tilt on the first test run.
		f.installTilt()
		installed = true
	}

	return f
}

func (f *fixture) testDirPath(s string) string {
	return filepath.Join(f.dir, s)
}

func (f *fixture) installTilt() {
	cmd := exec.CommandContext(f.ctx, "go", "install", "-mod", "vendor", "github.com/windmilleng/tilt/cmd/tilt")
	f.runOrFail(cmd, "Building tilt")
}

func (f *fixture) runOrFail(cmd *exec.Cmd, msg string) {
	// Use Output() instead of Run() because that captures Stderr in the ExitError.
	_, err := cmd.Output()
	if err == nil {
		return
	}

	exitErr, isExitErr := err.(*exec.ExitError)
	if isExitErr {
		f.t.Fatalf("%s\nError: %v\nStderr:\n%s\n", msg, err, string(exitErr.Stderr))
		return
	}
	f.t.Fatalf("%s. Error: %v", msg, err)
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
		case <-f.activeTiltDone():
			f.t.Fatalf("Tilt died while waiting: %v", f.activeTiltErr())
		case <-ctx.Done():
			f.t.Fatalf("Timed out waiting for expected result (%s)\n"+
				"Expected: %s\n"+
				"Actual: %s\n"+
				"Current error: %v\n",
				msg, expectedContents, actualContents, err)
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (f *fixture) activeTiltDone() <-chan struct{} {
	if f.activeTiltUp != nil {
		return f.activeTiltUp.Done()
	}
	neverDone := make(chan struct{})
	return neverDone
}

func (f *fixture) activeTiltErr() error {
	if f.activeTiltUp != nil {
		return f.activeTiltUp.Err()
	}
	return nil
}

func (f *fixture) LogWriter() io.Writer {
	return io.MultiWriter(f.logs, os.Stdout)
}

func (f *fixture) TiltUp(name string) {
	args := []string{"--watch=false"}
	if name != "" {
		args = append(args, name)
	}
	response, err := f.tilt.Up(args, f.LogWriter())
	if err != nil {
		f.t.Fatalf("TiltUp %s: %v", name, err)
	}
	select {
	case <-response.Done():
		err := response.Err()
		if err != nil {
			f.t.Fatalf("TiltUp %s: %v", name, err)
		}
	case <-f.ctx.Done():
		err := f.ctx.Err()
		if err != nil {
			f.t.Fatalf("TiltUp %s: %v", name, err)
		}
	}
}

func (f *fixture) TiltWatch() {
	response, err := f.tilt.Up(f.tiltArgs, f.LogWriter())
	if err != nil {
		f.t.Fatalf("TiltWatch: %v", err)
	}
	f.activeTiltUp = response
}

func (f *fixture) TiltWatchExec() {
	response, err := f.tilt.Up(append([]string{"--update-mode=exec"}, f.tiltArgs...), f.LogWriter())
	if err != nil {
		f.t.Fatalf("TiltWatchExec: %v", err)
	}
	f.activeTiltUp = response
}

func (f *fixture) ReplaceContents(fileBaseName, original, replacement string) {
	file := f.testDirPath(fileBaseName)
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
	if f.tearingDown {
		return
	}

	isTiltStillUp := f.activeTiltUp != nil && f.activeTiltUp.Err() == nil
	if f.t.Failed() && isTiltStillUp {
		fmt.Printf("Test failed, dumping engine state\n----\n")
		err := f.tilt.DumpEngine(os.Stdout)
		if err != nil {
			fmt.Printf("Error: %v", err)
		}
		fmt.Printf("\n----\n")

		err = f.activeTiltUp.KillAndDumpThreads()
		if err != nil {
			fmt.Printf("error killing tilt: %v\n", err)
		}
	}

	f.cancel()
	f.ctx = context.Background()
	f.tearingDown = true
}

func (f *fixture) KillProcs() {
	if f.activeTiltUp != nil {
		err := f.activeTiltUp.Kill()
		if err != nil {
			fmt.Printf("error killing tilt: %v\n", err)
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
	f.tilt.Environ["SKIP_NAMESPACE"] = "true"

	err := f.tilt.Down(os.Stdout)
	if err != nil {
		f.t.Errorf("Running tilt down: %v", err)
	}

	for k, v := range f.originalFiles {
		_ = ioutil.WriteFile(k, []byte(v), os.FileMode(0777))
	}
}

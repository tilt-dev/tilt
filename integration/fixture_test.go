// +build integration

package integration

import (
	"context"
	"fmt"
	"go/build"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/testutils/bufsync"
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
	skipTiltDown  bool
}

func newFixture(t *testing.T, dir string) *fixture {
	if dir == "" {
		// test doesn't require any in-repo assets, so chdir to a tempdir
		// to prevent accidentally overwriting repo files with Tilt commands
		dir = t.TempDir()
	} else {
		// checking for `..` is heavy-handed, but there's no valid reason for
		// an integration test to use it
		if filepath.IsAbs(dir) || strings.Contains(dir, "..") {
			t.Fatalf("dir %q should be a relative path under the integration/ directory", dir)
		}
		dir = filepath.Join(packageDir, dir)
	}
	err := os.Chdir(dir)
	if err != nil {
		t.Fatal(err)
	}

	client := NewTiltDriver(t, TiltDriverUseRandomFreePort)
	client.Environ["TILT_DISABLE_ANALYTICS"] = "true"

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
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
	f.t.Helper()
	// use the current GOROOT to pick which Go to build with
	goBin := filepath.Join(build.Default.GOROOT, "bin", "go")
	cmd := exec.CommandContext(f.ctx, goBin, "install", "-mod", "vendor", "github.com/tilt-dev/tilt/cmd/tilt")
	cmd.Dir = packageDir
	f.runOrFail(cmd, "Building tilt")
}

func (f *fixture) runOrFail(cmd *exec.Cmd, msg string) {
	f.t.Helper()
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

func (f *fixture) Curl(url string) (int, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return -1, "", errors.Wrap(err, "Curl")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		f.t.Errorf("Error fetching %s: %s", url, resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return -1, "", errors.Wrap(err, "Curl")
	}
	return resp.StatusCode, string(body), nil
}

func (f *fixture) CurlUntil(ctx context.Context, url string, expectedContents string) {
	f.t.Helper()
	f.WaitUntil(ctx, fmt.Sprintf("curl(%s)", url), func() (string, error) {
		_, body, err := f.Curl(url)
		return body, err
	}, expectedContents)
}

func (f *fixture) CurlUntilStatusCode(ctx context.Context, url string, expectedStatusCode int) {
	f.t.Helper()
	const prefix = "HTTP Status Code: "
	f.WaitUntil(ctx, fmt.Sprintf("curl(%s)", url), func() (string, error) {
		code, _, err := f.Curl(url)
		return prefix + strconv.Itoa(code), err
	}, prefix+strconv.Itoa(expectedStatusCode))
}

func (f *fixture) WaitUntil(ctx context.Context, msg string, fun func() (string, error), expectedContents string) {
	f.t.Helper()
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

func (f *fixture) TiltCI(args ...string) {
	err := f.tilt.CI(f.ctx, f.LogWriter(), args...)
	if err != nil {
		f.t.Fatalf("TiltCI: %v", err)
	}
}

func (f *fixture) TiltUp(args ...string) {
	response, err := f.tilt.Up(f.ctx, UpCommandUp, f.LogWriter(), args...)
	if err != nil {
		f.t.Fatalf("TiltUp: %v", err)
	}
	f.activeTiltUp = response
}

func (f *fixture) TiltDemo(args ...string) {
	response, err := f.tilt.Up(f.ctx, UpCommandDemo, f.LogWriter(), args...)
	if err != nil {
		f.t.Fatalf("TiltDemo: %v", err)
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
		fmt.Printf("Test failed, dumping internals\n----\n")
		fmt.Printf("Engine\n----\n")
		err := f.tilt.DumpEngine(f.ctx, os.Stdout)
		if err != nil {
			fmt.Printf("Error dumping engine: %v", err)
		}

		fmt.Printf("\n----\nAPI Server\n----\n")
		apiTypes, err := f.tilt.APIResources(f.ctx)
		if err != nil {
			fmt.Printf("Error determining available API resources: %v\n", err)
		} else {
			for _, apiType := range apiTypes {
				fmt.Printf("\n----\n%s\n----\n", strings.ToUpper(apiType))
				getOut, err := f.tilt.Get(f.ctx, apiType)
				fmt.Print(string(getOut))
				if err != nil {
					fmt.Printf("Error getting %s: %v", apiType, err)
				}
				fmt.Printf("\n----\n")
			}
		}

		err = f.activeTiltUp.KillAndDumpThreads()
		if err != nil {
			fmt.Printf("error killing tilt: %v\n", err)
		}
	}

	f.tearingDown = true
}

func (f *fixture) KillProcs() {
	if f.activeTiltUp != nil {
		err := f.activeTiltUp.TriggerExit()
		if err != nil && err.Error() != "os: process already finished" {
			fmt.Printf("error killing tilt: %v\n", err)
		}
	}
}

func (f *fixture) TearDown() {
	f.StartTearDown()

	// give `tilt up` a chance to exit gracefully
	// (once the context is canceled, it will be immediately SIGKILL'd)
	f.KillProcs()
	f.cancel()
	f.ctx = context.Background()

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

	if !f.skipTiltDown {
		ctx, cancel := context.WithTimeout(f.ctx, 30*time.Second)
		defer cancel()
		err := f.tilt.Down(ctx, os.Stdout)
		if err != nil {
			f.t.Errorf("Running tilt down: %v", err)
		}
	}

	for k, v := range f.originalFiles {
		_ = ioutil.WriteFile(k, []byte(v), os.FileMode(0777))
	}
}

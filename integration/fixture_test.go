package integration

import (
	"bytes"
	"context"
	"fmt"
	"go/build"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var packageDir string
var imageTagPrefix string

const namespaceFlag = "-n=tilt-integration"

func init() {
	pkg, err := build.Default.Import("github.com/windmilleng/tilt/integration", ".", build.FindOnly)
	if err != nil {
		panic(fmt.Sprintf("Could not find integration test source code: %v", err))
	}

	packageDir = pkg.Dir
	imageTagPrefix = fmt.Sprintf("tilt-T-%x-", time.Now().Unix())
}

type fixture struct {
	t             *testing.T
	ctx           context.Context
	cancel        func()
	dir           string
	logs          *bytes.Buffer
	cmds          []*exec.Cmd
	originalFiles map[string]string
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
		logs:          bytes.NewBuffer(nil),
		originalFiles: make(map[string]string),
	}
	return f
}

func (f *fixture) DumpLogs() {
	_, _ = os.Stdout.Write(f.logs.Bytes())
}

func (f *fixture) WaitUntil(ctx context.Context, msg string, fun func() (string, error), expectedContents string) {
	for {
		actualContents, err := fun()
		if err == nil && strings.Contains(actualContents, expectedContents) {
			return
		}

		select {
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

func (f *fixture) tiltCmd(tiltArgs []string, outWriter io.Writer) *exec.Cmd {
	outWriter = io.MultiWriter(f.logs, outWriter)

	gopath, ok := os.LookupEnv("GOPATH")
	if !ok {
		gopath = build.Default.GOPATH
	}
	args := []string{
		"run",
		filepath.Join(gopath, "src/github.com/windmilleng/tilt/cmd/tilt/main.go"),
	}
	args = append(args, tiltArgs...)
	cmd := exec.CommandContext(f.ctx, "go", args...)
	cmd.Stdout = outWriter
	cmd.Stderr = outWriter
	cmd.Env = append(os.Environ(), "TILT_DISABLE_ANALYTICS=true")
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

func (f *fixture) TiltWatch(name string) {
	cmd := f.tiltCmd([]string{"up", name, "--debug", "--hud=false", "--port=0"}, os.Stdout)
	err := cmd.Start()
	if err != nil {
		f.t.Fatal(err)
	}

	f.cmds = append(f.cmds, cmd)
	go func() {
		_ = cmd.Wait()
	}()
}

func (f *fixture) ReplaceContents(fileBaseName, original, replacement string) {
	file := filepath.Join(f.dir, fileBaseName)
	contents, ok := f.originalFiles[file]
	if !ok {
		contentsB, err := ioutil.ReadFile(file)
		if err != nil {
			f.t.Fatal(err)
		}
		contents = string(contentsB)
		f.originalFiles[file] = contents
	}

	newContents := strings.Replace(contents, original, replacement, -1)
	if newContents == contents {
		f.t.Fatalf("Could not find contents to replace in file %s: %s", fileBaseName, contents)
	}

	err := ioutil.WriteFile(file, []byte(newContents), os.FileMode(0777))
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) TearDown() {
	f.tearingDown = true

	for _, cmd := range f.cmds {
		process := cmd.Process
		if process != nil {
			process.Kill()
		}
	}

	f.cancel()

	for k, v := range f.originalFiles {
		_ = ioutil.WriteFile(k, []byte(v), os.FileMode(0777))
	}
}

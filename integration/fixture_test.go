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
	f.CreateNamespaceIfNecessary()
	f.ClearNamespace()
	return f
}

func (f *fixture) DumpLogs() {
	_, _ = os.Stdout.Write(f.logs.Bytes())
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
	cmd := f.tiltCmd([]string{"up", name, "--watch=false", "--debug", "--image-tag-prefix=" + imageTagPrefix}, out)
	err := cmd.Run()
	if err != nil {
		f.t.Fatalf("Failed to up service: %v. Logs:\n%s", err, out.String())
	}
}

func (f *fixture) TiltWatch(name string) {
	cmd := f.tiltCmd([]string{"up", name, "--debug"}, os.Stdout)
	err := cmd.Start()
	if err != nil {
		f.t.Fatal(err)
	}

	f.cmds = append(f.cmds, cmd)
	go func() {
		_ = cmd.Wait()
	}()
}

// Waits until all pods matching the selector are ready.
// At least one pod must match.
// Returns the names of the ready pods.
func (f *fixture) WaitForAllPodsReady(ctx context.Context, selector string) []string {
	for {
		allPodsReady, output, podNames := f.AllPodsReady(ctx, selector)
		if allPodsReady {
			return podNames
		}

		select {
		case <-ctx.Done():
			f.t.Fatalf("Timed out waiting for pods to be ready. Selector: %s. Output:\n:%s\n", selector, output)
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// Returns the output (for diagnostics) and the name of the ready pods.
func (f *fixture) AllPodsReady(ctx context.Context, selector string) (bool, string, []string) {
	cmd := exec.Command("kubectl", "get", "pods",
		namespaceFlag, "--selector="+selector, "-o=template",
		"--template", "{{range .items}}{{.metadata.name}} {{.status.phase}}{{println}}{{end}}")
	out, err := cmd.Output()
	if err != nil {
		f.t.Fatal(err)
	}

	outStr := string(out)
	lines := strings.Split(outStr, "\n")
	podNames := []string{}
	hasOneRunningPod := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		elements := strings.Split(line, " ")
		if len(elements) < 2 {
			f.t.Fatalf("Unexpected output of kubect get pods: %s", outStr)
		}

		name, phase := elements[0], elements[1]
		if phase == "Running" {
			hasOneRunningPod = true
		} else {
			return false, outStr, nil
		}

		podNames = append(podNames, name)
	}
	return hasOneRunningPod, outStr, podNames
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

	for k, v := range f.originalFiles {
		_ = ioutil.WriteFile(k, []byte(v), os.FileMode(0777))
	}
}

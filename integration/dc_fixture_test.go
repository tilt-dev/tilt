package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"strings"
	"testing"
)

type dcFixture struct {
	*fixture
}

func newDCFixture(t *testing.T, dir string) *dcFixture {
	f := newFixture(t, dir)
	return &dcFixture{fixture: f}
}

func (f *dcFixture) TearDown() {
	f.StartTearDown()
	f.dockerKillAll("tilt")
	f.fixture.TearDown()
}

func (f *dcFixture) dockerCmd(args []string, outWriter io.Writer) *exec.Cmd {
	outWriter = io.MultiWriter(f.logs, outWriter)
	cmd := exec.CommandContext(f.ctx, "docker", args...)
	cmd.Stdout = outWriter
	cmd.Stderr = outWriter
	return cmd
}

func (f *dcFixture) dockerCmdOutput(args []string) (string, error) {
	out := &bytes.Buffer{}
	cmd := f.dockerCmd(args, out)
	err := cmd.Run()
	return out.String(), err
}

func (f *dcFixture) dockerContainerID(name string) (string, error) {
	out := &bytes.Buffer{}
	cmd := f.dockerCmd([]string{
		"ps", "-q", "-f", fmt.Sprintf("name=%s", name),
	}, out)
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	containerID := strings.TrimSpace(out.String())
	if containerID == "" {
		return "", fmt.Errorf("No container found for %s", name)
	}
	return containerID, nil
}

func (f *dcFixture) dockerCreatedAt(name string) string {
	out := &bytes.Buffer{}
	cmd := f.dockerCmd([]string{
		"ps", "-q", "-f", fmt.Sprintf("name=%s", name), "--format", "{{.CreatedAt}}",
	}, out)
	err := cmd.Run()
	if err != nil {
		f.t.Fatal(fmt.Errorf("dockerCreatedAt failed: %v", err))
	}
	containerID := strings.TrimSpace(out.String())
	if containerID == "" {
		f.t.Fatal(fmt.Errorf("No container found for %s", name))
	}
	return containerID
}

func (f *dcFixture) dockerKillAll(name string) {
	out := &bytes.Buffer{}
	cmd := f.dockerCmd([]string{
		"ps", "-q", "-f", fmt.Sprintf("name=%s", name),
	}, out)
	err := cmd.Run()
	if err != nil {
		f.t.Fatal(err)
	}
	cIDs := strings.Split(strings.TrimSpace(out.String()), " ")
	if len(cIDs) == 0 || (len(cIDs) == 1 && cIDs[0] == "") {
		return
	}

	// Kill the containers and their networks. It's ok if the containers
	// don't exist anymore.
	cmd = f.dockerCmd(append([]string{
		"kill",
	}, cIDs...), ioutil.Discard)
	_ = cmd.Run()

	cmd = f.dockerCmd([]string{"network", "prune", "-f"}, ioutil.Discard)
	_ = cmd.Run()
}

func (f *dcFixture) CurlUntil(ctx context.Context, service string, url string, expectedContents string) {
	f.WaitUntil(ctx, fmt.Sprintf("curl(%s)", url), func() (string, error) {
		out := &bytes.Buffer{}
		cID, err := f.dockerContainerID(service)
		if err != nil {
			return "", err
		}

		cmd := f.dockerCmd([]string{
			"exec", cID, "curl", "-s", url,
		}, out)
		err = cmd.Run()
		return out.String(), err
	}, expectedContents)
}

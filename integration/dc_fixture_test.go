//go:build integration
// +build integration

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
	ret := &dcFixture{}
	t.Cleanup(ret.TearDown)
	ret.fixture = newFixture(t, dir)
	return ret
}

func (f *dcFixture) TearDown() {
	// Double check it's all dead
	f.dockerKillAll("tilt")
	_ = exec.CommandContext(f.ctx, "pkill", "docker-compose").Run()
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

func doV1V2(t *testing.T, body func(t *testing.T)) {
	t.Run("docker-compose-v1", func(t *testing.T) {
		t.Setenv("TILT_DOCKER_COMPOSE_CMD", "docker-compose-v1")
		fmt.Println("Running with docker-compose-v1")
		body(t)
	})

	cmd := exec.Command("docker-compose", "version")
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("error: getting docker-compose version: %v\n", err)
	} else {
		fmt.Print(string(out))
	}
	t.Run("docker-compose v2?", func(t *testing.T) {
		fmt.Println("Running with docker-compose")
		body(t)
		fmt.Print(string(out))
	})
}

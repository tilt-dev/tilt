package tiltd_server

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/docker/docker/client"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/tiltd"
)

type Daemon struct {
	b build.Builder
}

var _ tiltd.TiltD = &Daemon{}

func NewDaemon() (*Daemon, error) {
	opts := make([]func(*client.Client) error, 0)
	opts = append(opts, client.FromEnv)

	// Use client for docker 17
	// https://docs.docker.com/develop/sdk/#api-version-matrix
	// API version 1.30 is the first version where the full digest
	// shows up in the API output of BuildImage
	opts = append(opts, client.WithVersion("1.30"))
	dcli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, err
	}
	b := build.NewLocalDockerBuilder(dcli)
	return &Daemon{b: b}, nil
}

func (d *Daemon) CreateService(ctx context.Context, k8sYaml string, dockerfile string, mounts []build.Mount, steps []build.Cmd, dockerfileTag string, stdoutWriter io.Writer, stderrWriter io.Writer) error {
	// TODO(maia): a real entrypoint here

	digest, err := d.b.BuildDocker(ctx, dockerfile, mounts, steps, build.Cmd{})
	if err != nil {
		return err
	}
	err = d.b.PushDocker(ctx, dockerfileTag, digest)
	if err != nil {
		return err
	}

	entities, err := k8s.ParseYAMLFromString(k8sYaml)
	if err != nil {
		return err
	}

	didReplace := false
	newK8sEntities := []k8s.K8sEntity{}
	for _, e := range entities {
		newK8s, replaced, err := k8s.InjectImageDigestWithStrings(e, dockerfileTag, string(digest))
		if err != nil {
			return err
		}
		if replaced {
			didReplace = true
		}
		newK8sEntities = append(newK8sEntities, newK8s)
	}

	if !didReplace {
		return fmt.Errorf("Docker image missing from yaml: %s", dockerfileTag)
	}

	newYAMLString, err := k8s.SerializeYAML(newK8sEntities)
	if err != nil {
		return err
	}

	return k8s.Apply(ctx, newYAMLString, stdoutWriter, stderrWriter)

}

func RunDaemon(ctx context.Context) (*os.Process, error) {
	cmd := exec.CommandContext(ctx, os.Args[0], "daemon")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return nil, err
	}
	return cmd.Process, nil
}

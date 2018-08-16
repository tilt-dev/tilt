package engine

import (
	"context"
	"fmt"
	"os"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/client"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/image"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/wmclient/pkg/dirs"
)

type BuildToken interface{}

type BuildAndDeployer interface {
	BuildAndDeploy(ctx context.Context, service model.Service, token BuildToken) (BuildToken, error)
}

var _ BuildAndDeployer = localBuildAndDeployer{}

type localBuildAndDeployer struct {
	b       build.Builder
	history image.ImageHistory
}

func NewLocalBuildAndDeployer() (BuildAndDeployer, error) {
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
	dir, err := dirs.UseWindmillDir()
	if err != nil {
		return nil, err
	}
	history, err := image.NewImageHistory(dir)
	if err != nil {
		return nil, err
	}

	return localBuildAndDeployer{
		b:       b,
		history: history,
	}, nil
}

func (l localBuildAndDeployer) BuildAndDeploy(ctx context.Context, service model.Service, token BuildToken) (BuildToken, error) {
	checkpoint := l.history.CheckpointNow()
	name, err := reference.ParseNormalizedNamed(service.DockerfileTag)
	if err != nil {
		return nil, err
	}

	digest, err := l.b.BuildDocker(ctx, service.DockerfileText, service.Mounts, service.Steps, &service.Entrypoint)

	if err != nil {
		return nil, err
	}

	l.history.Add(name, digest, checkpoint)

	pushedDigest, err := l.b.PushDocker(ctx, name, digest)
	if err != nil {
		return nil, err
	}

	entities, err := k8s.ParseYAMLFromString(service.K8sYaml)
	if err != nil {
		return nil, err
	}

	didReplace := false
	newK8sEntities := []k8s.K8sEntity{}
	for _, e := range entities {
		newK8s, replaced, err := k8s.InjectImageDigest(e, name, pushedDigest)
		if err != nil {
			return nil, err
		}
		if replaced {
			didReplace = true
		}
		newK8sEntities = append(newK8sEntities, newK8s)
	}

	if !didReplace {
		return nil, fmt.Errorf("Docker image missing from yaml: %s", service.DockerfileTag)
	}

	newYAMLString, err := k8s.SerializeYAML(newK8sEntities)
	if err != nil {
		return nil, err
	}

	// TODO(matt) wire up logging to the grpc stream and drop the stdout/stderr args here
	return nil, k8s.Apply(ctx, newYAMLString, os.Stdout, os.Stderr)
}

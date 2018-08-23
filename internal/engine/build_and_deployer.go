package engine

import (
	"context"
	"fmt"

	"k8s.io/api/core/v1"

	"github.com/docker/distribution/reference"
	digest "github.com/opencontainers/go-digest"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/image"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
)

type buildToken struct {
	d digest.Digest
	n reference.Named
}

func (b *buildToken) isEmpty() bool {
	return b == nil
}

type BuildAndDeployer interface {
	// Builds and deployed the specified service.
	// Returns a buildToken that can be passed on successive calls to allow incremental builds.
	// If buildToken is passed and changedFiles is non-nil, changedFiles should specify the list of files that have
	//   changed since the last build.
	BuildAndDeploy(ctx context.Context, service model.Service, token *buildToken, changedFiles []string) (*buildToken, error)
}

var _ BuildAndDeployer = localBuildAndDeployer{}

type localBuildAndDeployer struct {
	b         build.Builder
	history   image.ImageHistory
	k8sClient k8s.Client
	env       k8s.Env
}

func NewLocalBuildAndDeployer(b build.Builder, k8sClient k8s.Client, history image.ImageHistory, env k8s.Env) (BuildAndDeployer, error) {
	return localBuildAndDeployer{
		b:         b,
		history:   history,
		k8sClient: k8sClient,
		env:       env,
	}, nil
}

func (l localBuildAndDeployer) BuildAndDeploy(ctx context.Context, service model.Service, token *buildToken, changedFiles []string) (*buildToken, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-BuildAndDeploy")
	defer span.Finish()
	checkpoint := l.history.CheckpointNow()

	var name reference.Named
	var d digest.Digest
	if token.isEmpty() {
		newDigest, err := l.b.BuildDockerFromScratch(ctx, build.Dockerfile(service.DockerfileText), service.Mounts, service.Steps, service.Entrypoint)
		if err != nil {
			return nil, err
		}
		d = newDigest

		name, err = reference.ParseNormalizedNamed(service.DockerfileTag)
		if err != nil {
			return nil, err
		}

	} else {
		// TODO(dmiller): in the future this shouldn't do a push, or a k8s apply, but for now it does
		newDigest, err := l.b.BuildDockerFromExisting(ctx, token.d, build.MountsToPath(service.Mounts), service.Steps)
		if err != nil {
			return nil, err
		}
		d = newDigest
		name = token.n
	}

	logger.Get(ctx).Verbosef("(Adding checkpoint to history)")
	err := l.history.AddAndPersist(ctx, name, d, checkpoint, service)
	if err != nil {
		return nil, err
	}

	var refToInject reference.Named

	// If we're using docker-for-desktop as our k8s backend,
	// we don't need to push to the central registry.
	// The k8s will use the image already available
	// in the local docker daemon.
	canSkipPush := l.env == k8s.EnvDockerDesktop
	if canSkipPush {
		refToInject, err = l.b.TagDocker(ctx, name, d)
		if err != nil {
			return nil, err
		}
	} else {
		refToInject, err = l.b.PushDocker(ctx, name, d)
		if err != nil {
			return nil, err
		}
	}

	logger.Get(ctx).Infof("Parsing Kubernetes config YAML")
	entities, err := k8s.ParseYAMLFromString(service.K8sYaml)
	if err != nil {
		return nil, err
	}

	didReplace := false
	newK8sEntities := []k8s.K8sEntity{}
	for _, e := range entities {
		// For development, image pull policy should never be set to "Always",
		// even if it might make sense to use "Always" in prod. People who
		// set "Always" for development are shooting their own feet.
		e, err = k8s.InjectImagePullPolicy(e, v1.PullIfNotPresent)
		if err != nil {
			return nil, err
		}

		// When working with a local k8s cluster, we set the pull policy to Never,
		// to ensure that k8s fails hard if the image is missing from docker.
		policy := v1.PullIfNotPresent
		if canSkipPush {
			policy = v1.PullNever
		}
		e, replaced, err := k8s.InjectImageDigest(e, refToInject, policy)
		if err != nil {
			return nil, err
		}
		if replaced {
			didReplace = true
		}
		newK8sEntities = append(newK8sEntities, e)
	}

	if !didReplace {
		return nil, fmt.Errorf("Docker image missing from yaml: %s", service.DockerfileTag)
	}

	newYAMLString, err := k8s.SerializeYAML(newK8sEntities)
	if err != nil {
		return nil, err
	}

	return &buildToken{d: d, n: name}, l.k8sClient.Apply(ctx, newYAMLString)
}

package build

import (
	"flag"
	"io"

	"github.com/docker/cli/opts"

	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func Options(archive io.Reader, spec v1alpha1.DockerImageSpec) docker.BuildOptions {
	return docker.BuildOptions{
		Context:     archive,
		Dockerfile:  "Dockerfile",
		Remove:      shouldRemoveImage(),
		BuildArgs:   opts.ConvertKVStringsToMapWithNil(spec.Args),
		Target:      spec.Target,
		SSHSpecs:    spec.SSHAgentConfigs,
		Network:     spec.Network,
		ExtraTags:   spec.ExtraTags,
		SecretSpecs: spec.Secrets,
		CacheFrom:   spec.CacheFrom,
		PullParent:  spec.Pull,
		Platform:    spec.Platform,
	}
}

func shouldRemoveImage() bool {
	return flag.Lookup("test.v") != nil
}

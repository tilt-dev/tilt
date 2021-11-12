package build

import (
	"flag"
	"io"

	"github.com/docker/cli/opts"

	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/pkg/model"
)

func Options(archive io.Reader, db model.DockerBuild) docker.BuildOptions {
	return docker.BuildOptions{
		Context:     archive,
		Dockerfile:  "Dockerfile",
		Remove:      shouldRemoveImage(),
		BuildArgs:   opts.ConvertKVStringsToMapWithNil(db.Args),
		Target:      db.Target,
		SSHSpecs:    db.SSHAgentConfigs,
		Network:     db.Network,
		ExtraTags:   db.ExtraTags,
		SecretSpecs: db.Secrets,
		CacheFrom:   db.CacheFrom,
		PullParent:  db.Pull,
		Platform:    db.Platform,
	}
}

func shouldRemoveImage() bool {
	return flag.Lookup("test.v") != nil
}

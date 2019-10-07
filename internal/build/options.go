package build

import (
	"flag"
	"io"

	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/pkg/model"
)

var BuiltByTiltLabel = map[string]string{"builtby": "tilt"}

func Options(archive io.Reader, args model.DockerBuildArgs, target model.DockerBuildTarget) docker.BuildOptions {
	return docker.BuildOptions{
		Context:    archive,
		Dockerfile: "Dockerfile",
		Remove:     shouldRemoveImage(),
		BuildArgs:  manifestBuildArgsToDockerBuildArgs(args),
		Target:     string(target),
		Labels:     BuiltByTiltLabel,
	}
}

func shouldRemoveImage() bool {
	return flag.Lookup("test.v") != nil
}

func manifestBuildArgsToDockerBuildArgs(args model.DockerBuildArgs) map[string]*string {
	r := make(map[string]*string, len(args))
	for k, a := range args {
		tmp := a
		r[k] = &tmp
	}

	return r
}

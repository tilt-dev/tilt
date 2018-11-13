package build

import (
	"flag"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/windmilleng/tilt/internal/model"
)

func Options(archive io.Reader, args model.DockerBuildArgs) types.ImageBuildOptions {
	return types.ImageBuildOptions{
		Context:    archive,
		Dockerfile: "Dockerfile",
		Remove:     shouldRemoveImage(),
		Version:    types.BuilderBuildKit,
		BuildArgs:  manifestBuildArgsToDockerBuildArgs(args),
	}
}

func shouldRemoveImage() bool {
	if flag.Lookup("test.v") == nil {
		return false
	}
	return true
}

func manifestBuildArgsToDockerBuildArgs(args model.DockerBuildArgs) map[string]*string {
	r := make(map[string]*string, len(args))
	for k, a := range args {
		r[k] = &a
	}

	return r
}

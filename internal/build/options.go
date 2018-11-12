package build

import (
	"flag"
	"io"

	"github.com/docker/docker/api/types"
)

func Options(archive io.Reader, args map[string]string) types.ImageBuildOptions {
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

func manifestBuildArgsToDockerBuildArgs(args map[string]string) map[string]*string {
	r := make(map[string]*string, len(args))
	for k, a := range args {
		r[k] = &a
	}

	return r
}

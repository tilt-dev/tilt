package build

import (
	"flag"
	"io"

	"github.com/docker/docker/api/types"
)

func Options(archive io.Reader) types.ImageBuildOptions {
	return types.ImageBuildOptions{
		Context:    archive,
		Dockerfile: "Dockerfile",
		Remove:     shouldRemoveImage(),
		Version:    types.BuilderBuildKit,
	}
}

func shouldRemoveImage() bool {
	if flag.Lookup("test.v") == nil {
		return false
	}
	return true
}

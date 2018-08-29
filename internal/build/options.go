package build

import (
	"bytes"
	"flag"

	"github.com/docker/docker/api/types"
)

func Options(archive *bytes.Reader) types.ImageBuildOptions {
	return types.ImageBuildOptions{
		Context:    archive,
		Dockerfile: "Dockerfile",
		Remove:     shouldRemoveImage(),
		// TODO(dmiller): parameterize this via a cli flag
		//Version:    types.BuilderBuildKit,
	}
}

func shouldRemoveImage() bool {
	if flag.Lookup("test.v") == nil {
		return false
	}
	return true
}

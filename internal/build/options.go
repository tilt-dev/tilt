package build

import (
	"flag"
	"io"
	"os"

	"github.com/docker/docker/api/types"
)

func Options(archive io.Reader) types.ImageBuildOptions {
	useBuildkit := os.Getenv("TILT_BUILDKIT")
	version := types.BuilderV1
	if useBuildkit == "1" {
		version = types.BuilderBuildKit
	}

	return types.ImageBuildOptions{
		Context:    archive,
		Dockerfile: "Dockerfile",
		Remove:     shouldRemoveImage(),
		Version:    version,
	}
}

func shouldRemoveImage() bool {
	if flag.Lookup("test.v") == nil {
		return false
	}
	return true
}

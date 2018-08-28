// +build windows

package containerizedengine

import (
	"github.com/containerd/containerd"
	"github.com/docker/cli/internal/pkg/containerized"
)

func genSpec() containerd.NewContainerOpts {
	return containerd.WithSpec(&engineSpec,
		containerized.WithAllCapabilities,
	)
}

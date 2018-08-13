package tiltd

import (
	"context"

	"github.com/windmilleng/tilt/internal/build"
)

const Port = 10000

type TiltD interface {
	CreateService(ctx context.Context, k8sYaml string, dockerFileText string, mounts []build.Mount, steps []build.Cmd, dockerfileTag string) error
}

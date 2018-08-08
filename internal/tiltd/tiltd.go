package tiltd

import "context"

const Port = 10000

type TiltD interface {
	CreateService(ctx context.Context, k8sYaml string) error
}

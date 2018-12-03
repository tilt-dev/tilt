package tiltfile2

import (
	"github.com/windmilleng/tilt/internal/k8s"
)

// k8sResource
type k8sResource struct {
	name     string
	k8s      []k8s.K8sEntity
	imageRef string

	// FIXME(dbentley): port forwards

	expandedFrom string // this resource was not declared but expanded from another resource
}

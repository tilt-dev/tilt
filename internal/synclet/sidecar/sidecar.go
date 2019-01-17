package sidecar

import (
	"fmt"

	"github.com/windmilleng/tilt/internal/container"
	"k8s.io/apimachinery/pkg/api/resource"

	"k8s.io/api/core/v1"
)

func syncletPrivileged() *bool {
	val := true
	return &val
}

// When we deploy Tilt for development, we override this with LDFLAGS
var SyncletTag = "v20190117"

const SyncletImageName = "gcr.io/windmill-public-containers/tilt-synclet"
const SyncletContainerName = "tilt-synclet"

var SyncletImageRef = container.MustParseNamed(SyncletImageName)

var SyncletContainer = v1.Container{
	Name:            SyncletContainerName,
	Image:           fmt.Sprintf("%s:%s", SyncletImageName, SyncletTag),
	ImagePullPolicy: v1.PullIfNotPresent,
	Resources:       v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("0Mi")}},
	VolumeMounts: []v1.VolumeMount{
		v1.VolumeMount{
			Name:      "tilt-dockersock",
			MountPath: "/var/run/docker.sock",
		},
	},
	SecurityContext: &v1.SecurityContext{
		Privileged: syncletPrivileged(),
	},
}

var SyncletVolume = v1.Volume{
	Name: "tilt-dockersock",
	VolumeSource: v1.VolumeSource{
		HostPath: &v1.HostPathVolumeSource{
			Path: "/var/run/docker.sock",
		},
	},
}

func PodSpecContainsSynclet(spec v1.PodSpec) bool {
	for _, container := range spec.Containers {
		if container.Name == SyncletContainerName {
			return true
		}
	}
	return false
}

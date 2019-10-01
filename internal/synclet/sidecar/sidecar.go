package sidecar

import (
	"fmt"
	"os"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/windmilleng/tilt/internal/container"
)

func syncletPrivileged() *bool {
	val := true
	return &val
}

const defaultSyncletImageName = "gcr.io/windmill-public-containers/tilt-synclet"

func getImageName() string {
	v := os.Getenv(SyncletImageEnvVar)
	if v == "" {
		return defaultSyncletImageName
	}
	return v
}

// When we deploy Tilt for development, we override this with LDFLAGS
var SyncletTag = "v20190904"

const SyncletImageEnvVar = "TILT_SYNCLET_IMAGE"

// TODO(nick): Revamp this to not rely on global init
var SyncletImageName = getImageName()

const SyncletContainerName = "tilt-synclet"

var SyncletImageRef = container.MustParseNamed(SyncletImageName)

var SyncletContainer = v1.Container{
	Name:            SyncletContainerName,
	Image:           fmt.Sprintf("%s:%s", getImageName(), SyncletTag),
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

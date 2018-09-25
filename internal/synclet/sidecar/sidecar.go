package sidecar

import (
	"fmt"

	"k8s.io/api/core/v1"
)

func syncletPrivileged() *bool {
	val := true
	return &val
}

// When we deploy Tilt, we override this with LDFLAGS
const SyncletTag = "latest"

const SyncletImageName = "gcr.io/windmill-public-containers/tilt-synclet"

var SyncletContainer = v1.Container{
	Name:            "tilt-synclet",
	Image:           fmt.Sprintf("%s:%s", SyncletImageName, SyncletTag),
	ImagePullPolicy: v1.PullIfNotPresent,
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

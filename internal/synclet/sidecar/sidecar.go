package sidecar

import (
	"context"
	"fmt"
	"os"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/docker/distribution/reference"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/pkg/logger"
)

func syncletPrivileged() *bool {
	val := true
	return &val
}

const DefaultSyncletImageName = "gcr.io/windmill-public-containers/tilt-synclet"

// When we deploy Tilt for development, we override this with LDFLAGS
var SyncletTag = "v20190904"

const SyncletImageEnvVar = "TILT_SYNCLET_IMAGE"

const SyncletContainerName = "tilt-synclet"

type SyncletImageRef reference.NamedTagged
type SyncletContainer *v1.Container

func ProvideSyncletImageRef(ctx context.Context) (SyncletImageRef, error) {
	v := os.Getenv(SyncletImageEnvVar)
	if v != "" {
		logger.Get(ctx).Infof("Read %s from environment: %v", SyncletImageEnvVar, v)
		return syncletImageRefFromName(v)
	}
	return syncletImageRefFromName(DefaultSyncletImageName)
}

func syncletImageRefFromName(imageName string) (SyncletImageRef, error) {
	ref, err := container.ParseNamedTagged(fmt.Sprintf("%s:%s", imageName, SyncletTag))
	if err != nil {
		return nil, err
	}
	return SyncletImageRef(ref), nil
}

func ProvideSyncletContainer(ref SyncletImageRef) SyncletContainer {
	return SyncletContainer(&v1.Container{
		Name:            SyncletContainerName,
		Image:           ref.String(),
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
	})
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

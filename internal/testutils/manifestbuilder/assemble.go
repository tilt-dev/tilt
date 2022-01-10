package manifestbuilder

import (
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Assemble these targets into a manifest, that deploys to k8s,
// wiring up all the dependency ids so that the K8sTarget depends on all
// the deployed image targets
func assembleK8s(m model.Manifest, k model.K8sTarget, iTargets ...model.ImageTarget) model.Manifest {
	// images on which another image depends -- we assume they are base
	// images, i.e. not deployed directly, and so the deploy target
	// should not depend on them.
	baseImages := make(map[string]bool)
	for _, iTarget := range iTargets {
		for _, id := range iTarget.ImageMapDeps() {
			baseImages[id] = true
		}
	}

	imageMapNames := make([]string, 0, len(iTargets))
	for _, iTarget := range iTargets {
		if baseImages[iTarget.ImageMapName()] {
			continue
		}
		imageMapNames = append(imageMapNames, iTarget.ImageMapName())
	}
	k = k.WithImageDependencies(model.FilterLiveUpdateOnly(imageMapNames, iTargets))
	return m.
		WithImageTargets(iTargets).
		WithDeployTarget(k)
}

// Assemble these targets into a manifest, that deploys to docker compose,
// wiring up all the dependency ids so that the DockerComposeTarget depends on all
// the deployed image targets
func assembleDC(m model.Manifest, dcTarg model.DockerComposeTarget, iTargets ...model.ImageTarget) model.Manifest {
	// images on which another image depends -- we assume they are base
	// images, i.e. not deployed directly, and so the deploy target
	// should not depend on them.
	baseImages := make(map[string]bool)
	for _, iTarget := range iTargets {
		for _, id := range iTarget.ImageMapDeps() {
			baseImages[id] = true
		}
	}

	imageMapNames := make([]string, 0, len(iTargets))
	for _, iTarget := range iTargets {
		if baseImages[iTarget.ImageMapName()] {
			continue
		}
		imageMapNames = append(imageMapNames, iTarget.ImageMapName())
	}

	if len(imageMapNames) == 0 {
		iTarget := model.ImageTarget{
			ImageMapSpec: v1alpha1.ImageMapSpec{
				Selector: dcTarg.Spec.Service,
			},
			BuildDetails: model.DockerComposeBuild{
				Service: dcTarg.Spec.Service,
			},
		}
		imageMapNames = append(imageMapNames, iTarget.ImageMapName())
		iTargets = append(iTargets, iTarget)
	}

	dc := dcTarg.WithImageMapDeps(model.FilterLiveUpdateOnly(imageMapNames, iTargets))
	return m.
		WithImageTargets(iTargets).
		WithDeployTarget(dc)
}

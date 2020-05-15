package manifestbuilder

import "github.com/tilt-dev/tilt/pkg/model"

// Assemble these targets into a manifest, that deploys to k8s,
// wiring up all the dependency ids so that the K8sTarget depends on all
// the deployed image targets
func assembleK8s(m model.Manifest, k model.K8sTarget, iTargets ...model.ImageTarget) model.Manifest {
	// images on which another image depends -- we assume they are base
	// images, i.e. not deployed directly, and so the deploy target
	// should not depend on them.
	baseImages := make(map[model.TargetID]bool)
	for _, iTarget := range iTargets {
		for _, id := range iTarget.DependencyIDs() {
			baseImages[id] = true
		}
	}

	ids := make([]model.TargetID, 0, len(iTargets))
	for _, iTarget := range iTargets {
		if baseImages[iTarget.ID()] {
			continue
		}
		ids = append(ids, iTarget.ID())
	}
	k = k.WithDependencyIDs(ids)
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
	baseImages := make(map[model.TargetID]bool)
	for _, iTarget := range iTargets {
		for _, id := range iTarget.DependencyIDs() {
			baseImages[id] = true
		}
	}

	ids := make([]model.TargetID, 0, len(iTargets))
	for _, iTarget := range iTargets {
		if baseImages[iTarget.ID()] {
			continue
		}
		ids = append(ids, iTarget.ID())
	}
	dc := dcTarg.WithDependencyIDs(ids)
	return m.
		WithImageTargets(iTargets).
		WithDeployTarget(dc)
}

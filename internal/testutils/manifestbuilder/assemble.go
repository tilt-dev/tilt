package manifestbuilder

import "github.com/windmilleng/tilt/internal/model"

// Assemble these targets into a manifest, that deploys to k8s,
// wiring up all the dependency ids so that the K8sTarget depends on all
// the image targets
func assembleK8s(m model.Manifest, k model.K8sTarget, iTargets ...model.ImageTarget) model.Manifest {
	ids := make([]model.TargetID, 0, len(iTargets))
	for _, iTarget := range iTargets {
		ids = append(ids, iTarget.ID())
	}
	k = k.WithDependencyIDs(ids)
	return m.
		WithImageTargets(iTargets).
		WithDeployTarget(k)
}

// Assemble these targets into a manifest, that deploys to docker compose,
// wiring up all the dependency ids so that the DockerComposeTarget depends on all
// the image targets
func assembleDC(m model.Manifest, dcTarg model.DockerComposeTarget, iTargets ...model.ImageTarget) model.Manifest {
	ids := make([]model.TargetID, 0, len(iTargets))
	for _, iTarget := range iTargets {
		ids = append(ids, iTarget.ID())
	}
	dc := dcTarg.WithDependencyIDs(ids)
	return m.
		WithImageTargets(iTargets).
		WithDeployTarget(dc)
}

package buildcontrol

import "github.com/tilt-dev/tilt/pkg/model"

// Extract target specs from a manifest for BuildAndDeploy.
func BuildTargets(manifest model.Manifest) []model.TargetSpec {
	var result []model.TargetSpec

	for _, iTarget := range manifest.ImageTargets {
		result = append(result, iTarget)
	}

	if manifest.IsDC() {
		result = append(result, manifest.DockerComposeTarget())
	} else if manifest.IsK8s() {
		result = append(result, manifest.K8sTarget())
	} else if manifest.IsLocal() {
		result = append(result, manifest.LocalTarget())
	}

	return result
}

package model

func ExtractK8sTargets(specs []TargetSpec) []K8sTarget {
	kTargets := make([]K8sTarget, 0)
	for _, spec := range specs {
		t, ok := spec.(K8sTarget)
		if !ok {
			continue
		}
		kTargets = append(kTargets, t)
	}
	return kTargets
}

func ExtractImageTargets(specs []TargetSpec) []ImageTarget {
	iTargets := make([]ImageTarget, 0)
	for _, spec := range specs {
		t, ok := spec.(ImageTarget)
		if !ok {
			continue
		}
		iTargets = append(iTargets, t)
	}
	return iTargets
}

func ExtractDockerComposeTargets(specs []TargetSpec) []DockerComposeTarget {
	targets := make([]DockerComposeTarget, 0)
	for _, spec := range specs {
		t, ok := spec.(DockerComposeTarget)
		if !ok {
			continue
		}
		targets = append(targets, t)
	}
	return targets
}

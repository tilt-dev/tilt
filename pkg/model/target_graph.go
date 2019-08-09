package model

import "fmt"

type TargetGraph struct {
	// Targets in topological order.
	sortedTargets []TargetSpec
	byID          map[TargetID]TargetSpec
}

func NewTargetGraph(targets []TargetSpec) (TargetGraph, error) {
	sortedTargets, err := TopologicalSort(targets)
	if err != nil {
		return TargetGraph{}, err
	}

	return TargetGraph{
		sortedTargets: sortedTargets,
		byID:          MakeTargetMap(sortedTargets),
	}, nil
}

// In Tilt, Manifests should always be DAGs with a single root node
// (the deploy target). This is just a quick sanity check to make sure
// that's true, because many of our graph-traversal algorithms won't work if
// it's not true.
func (g TargetGraph) IsSingleSourceDAG() bool {
	seenIDs := make(map[TargetID]bool, len(g.sortedTargets))
	lastIdx := len(g.sortedTargets) - 1
	for i := lastIdx; i >= 0; i-- {
		t := g.sortedTargets[i]
		id := t.ID()
		isLastTarget := i == lastIdx
		if !isLastTarget && !seenIDs[id] {
			return false
		}

		for _, depID := range t.DependencyIDs() {
			seenIDs[depID] = true
		}
	}
	return true
}

// Visit t and its transitive dependencies in post-order (aka depth-first)
func (g TargetGraph) VisitTree(root TargetSpec, visit func(dep TargetSpec) error) error {
	visitedIDs := make(map[TargetID]bool)

	// pre-declare the variable, so that this function can recurse
	var helper func(current TargetSpec) error
	helper = func(current TargetSpec) error {
		deps, err := g.DepsOf(current)
		if err != nil {
			return err
		}

		for _, dep := range deps {
			if visitedIDs[dep.ID()] {
				continue
			}

			err := helper(dep)
			if err != nil {
				return err
			}
		}

		err = visit(current)
		if err != nil {
			return err
		}
		visitedIDs[current.ID()] = true
		return nil
	}

	return helper(root)
}

// Return the direct dependency targets.
func (g TargetGraph) DepsOf(t TargetSpec) ([]TargetSpec, error) {
	depIDs := t.DependencyIDs()
	result := make([]TargetSpec, len(depIDs))
	for i, depID := range depIDs {
		dep, ok := g.byID[depID]
		if !ok {
			return nil, fmt.Errorf("Dep %q not found in graph", depID)
		}
		result[i] = dep
	}
	return result, nil
}

// Is this image directly deployed a container?
func (g TargetGraph) IsDeployedImage(iTarget ImageTarget) bool {
	id := iTarget.ID()
	for _, t := range g.sortedTargets {
		switch t := t.(type) {
		case K8sTarget, DockerComposeTarget:
			// Returns true if a K8s or DC target directly depends on this image.
			for _, depID := range t.DependencyIDs() {
				if depID == id {
					return true
				}
			}
		}
	}
	return false
}

func (g TargetGraph) Images() []ImageTarget {
	return ExtractImageTargets(g.sortedTargets)
}

// Returns all the images in the graph that are directly deployed to a container.
func (g TargetGraph) DeployedImages() []ImageTarget {
	result := []ImageTarget{}
	for _, iTarget := range g.Images() {
		if g.IsDeployedImage(iTarget) {
			result = append(result, iTarget)
		}
	}
	return result
}

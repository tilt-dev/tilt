package model

import (
	"fmt"

	"github.com/pkg/errors"
)

// An abstract build graph of targets and their dependencies.
// Each target should have a unique ID.

type TargetType string
type TargetName string

func (n TargetName) String() string { return string(n) }

const (
	// Deployed k8s entities
	TargetTypeK8s TargetType = "k8s"

	// Image builds
	TargetTypeImage TargetType = "image"

	// Docker-compose service build and deploy
	// TODO(nick): Currently, build and deploy are represented as a single target.
	// In the future, we might have a separate build target and deploy target.
	TargetTypeDockerCompose TargetType = "docker-compose"

	// Runs a local command when triggered (manually or via changed dep)
	TargetTypeLocal TargetType = "local"

	// Aggregation of multiple targets into one UI view.
	// TODO(nick): Currently used as the type for both Manifest and YAMLManifest, though
	// we expect YAMLManifest to go away.
	TargetTypeManifest TargetType = "manifest"

	// Changes that affect all targets, rebuilding the target graph.
	TargetTypeConfigs TargetType = "configs"
)

type TargetID struct {
	Type TargetType
	Name TargetName
}

func (id TargetID) Empty() bool {
	return id.Type == "" || id.Name == ""
}

func (id TargetID) String() string {
	if id.Empty() {
		return ""
	}
	return fmt.Sprintf("%s:%s", id.Type, id.Name)
}

func TargetIDSet(tids []TargetID) map[TargetID]bool {
	res := make(map[TargetID]bool)
	for _, id := range tids {
		res[id] = true
	}
	return res
}

type TargetSpec interface {
	ID() TargetID

	// Check to make sure the spec is well-formed.
	// All TargetSpecs should throw an error in the case where the ID is empty.
	Validate() error

	DependencyIDs() []TargetID
}

type TargetStatus interface {
	TargetID() TargetID
	LastBuild() BuildRecord
}

type Target interface {
	Spec() TargetSpec
	Status() TargetStatus
}

// De-duplicate target ids, maintaining the same order.
func DedupeTargetIDs(ids []TargetID) []TargetID {
	result := make([]TargetID, 0, len(ids))
	dupes := make(map[TargetID]bool, len(ids))
	for _, id := range ids {
		if !dupes[id] {
			dupes[id] = true
			result = append(result, id)
		}
	}
	return result
}

// Map all the targets by their target ID.
func MakeTargetMap(targets []TargetSpec) map[TargetID]TargetSpec {
	result := make(map[TargetID]TargetSpec, len(targets))
	for _, target := range targets {
		result[target.ID()] = target
	}
	return result
}

// Create a topologically sorted list of targets. Returns an error
// if the targets can't be topologically sorted. (e.g., there's a cycle).
func TopologicalSort(targets []TargetSpec) ([]TargetSpec, error) {
	targetMap := MakeTargetMap(targets)
	result := make([]TargetSpec, 0, len(targets))
	inResult := make(map[TargetID]bool, len(targets))
	searching := make(map[TargetID]bool, len(targets))

	var ensureInResult func(id TargetID) error
	ensureInResult = func(id TargetID) error {
		if inResult[id] {
			return nil
		}
		if searching[id] {
			return fmt.Errorf("Found a cycle at target: %s", id.Name)
		}
		searching[id] = true

		current, ok := targetMap[id]
		if !ok {
			return fmt.Errorf("Missing target dependency: %s", id.Name)
		}

		for _, depID := range current.DependencyIDs() {
			err := ensureInResult(depID)
			if err != nil {
				return err
			}
		}
		result = append(result, current)
		inResult[id] = true
		return nil
	}

	for _, target := range targets {
		err := ensureInResult(target.ID())
		if err != nil {
			return nil, errors.Wrap(err, "Internal error")
		}
	}
	return result, nil
}

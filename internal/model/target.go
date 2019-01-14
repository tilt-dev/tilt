package model

import "fmt"

// An abstract build graph of targets and their dependencies.
// Each target should have a unique ID.

type TargetType string
type TargetName string

func (n TargetName) String() string { return string(n) }

const (
	// Deployed k8s entities
	TargetTypeK8s TargetType = "k8s"

	// Image builds
	// TODO(nick): It might make sense to represent FastBuild and normal Docker builds
	// as separate types.
	TargetTypeImage TargetType = "image"

	// Docker-compose service build and deploy
	// TODO(nick): Currently, build and deploy are represented as a single target.
	// In the future, we might have a separate build target and deploy target.
	TargetTypeDockerCompose TargetType = "docker-compose"

	// Aggregation of multiple targets into one UI view.
	// TODO(nick): Currenly used as the type for both Manifest and YAMLManifest, though
	// we expect YAMLManifest to go away.
	TargetTypeManifest TargetType = "manifest"
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

type TargetSpec interface {
	ID() TargetID

	// Check to make sure the spec is well-formed.
	// All TargetSpecs should throw an error in the case where the ID is empty.
	Validate() error

	// TODO(nick): Add dependency IDs
}

type TargetStatus interface {
	TargetID() TargetID
	ActiveBuild() BuildStatus
	LastBuild() BuildStatus
}

type Target interface {
	Spec() TargetSpec
	Status() TargetStatus
}

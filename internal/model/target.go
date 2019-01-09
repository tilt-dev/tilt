package model

import "fmt"

// An abstract build graph of targets and their dependencies.
// Each target should have a unique ID.

type TargetType string

const (
	TargetTypeK8s           TargetType = "k8s"            // Deployed k8s entities
	TargetTypeImage         TargetType = "image"          // Image builds
	TargetTypeDockerCompose TargetType = "docker-compose" // Docker-compose service build+deploy
)

type TargetID struct {
	Type TargetType
	Name string
}

func (id TargetID) String() string {
	return fmt.Sprintf("%s:%s", id.Type, id.Name)
}

type Target interface {
	ID() TargetID

	// TODO(nick): Add dependency IDs
}

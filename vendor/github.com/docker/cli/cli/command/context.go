package command

import (
	"errors"

	"github.com/docker/cli/cli/context/store"
)

// DockerContext is a typed representation of what we put in Context metadata
type DockerContext struct {
	Description       string       `json:",omitempty"`
	StackOrchestrator Orchestrator `json:",omitempty"`
}

// GetDockerContext extracts metadata from stored context metadata
func GetDockerContext(storeMetadata store.Metadata) (DockerContext, error) {
	if storeMetadata.Metadata == nil {
		// can happen if we save endpoints before assigning a context metadata
		// it is totally valid, and we should return a default initialized value
		return DockerContext{}, nil
	}
	res, ok := storeMetadata.Metadata.(DockerContext)
	if !ok {
		return DockerContext{}, errors.New("context metadata is not a valid DockerContext")
	}
	return res, nil
}

// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.21

package command

import (
	"encoding/json"
	"errors"

	"github.com/docker/cli/cli/context/store"
)

// DockerContext is a typed representation of what we put in Context metadata
type DockerContext struct {
	Description      string
	AdditionalFields map[string]any
}

// MarshalJSON implements custom JSON marshalling
func (dc DockerContext) MarshalJSON() ([]byte, error) {
	s := map[string]any{}
	if dc.Description != "" {
		s["Description"] = dc.Description
	}
	if dc.AdditionalFields != nil {
		for k, v := range dc.AdditionalFields {
			s[k] = v
		}
	}
	return json.Marshal(s)
}

// UnmarshalJSON implements custom JSON marshalling
func (dc *DockerContext) UnmarshalJSON(payload []byte) error {
	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}
	for k, v := range data {
		switch k {
		case "Description":
			dc.Description = v.(string)
		default:
			if dc.AdditionalFields == nil {
				dc.AdditionalFields = make(map[string]any)
			}
			dc.AdditionalFields[k] = v
		}
	}
	return nil
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
	if storeMetadata.Name == DefaultContextName {
		res.Description = "Current DOCKER_HOST based configuration"
	}
	return res, nil
}

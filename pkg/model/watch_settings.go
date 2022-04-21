package model

import "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

type WatchSettings struct {
	Ignores []Dockerignore
}

func (ws WatchSettings) Empty() bool {
	return len(ws.Ignores) == 0
}

type Dockerignore struct {
	// The path to evaluate the dockerignore contents relative to
	LocalPath string

	// A human-readable string that identifies where the ignores come from.
	Source string

	// Patterns parsed out of the .dockerignore file.
	Patterns []string
}

func (d Dockerignore) Empty() bool {
	return len(d.Patterns) == 0
}

func DockerignoresToIgnores(source []Dockerignore) []v1alpha1.IgnoreDef {
	result := make([]v1alpha1.IgnoreDef, 0, len(source))
	for _, s := range source {
		if s.Empty() {
			continue
		}
		result = append(result, v1alpha1.IgnoreDef{
			BasePath: s.LocalPath,
			Patterns: s.Patterns,
		})
	}
	return result
}

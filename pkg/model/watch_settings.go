package model

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

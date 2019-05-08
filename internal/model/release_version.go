package model

import "time"

type ReleaseVersion struct {
	// e.g., v0.8.1
	VersionNumber string

	PublishedAt time.Time
}

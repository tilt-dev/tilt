package store

import (
	"time"
)

// Helper functions for comparing times in the store.
// On Windows, time instants aren't monotonic, so we need to be
// more tolerant of equal times.
func AfterOrEqual(a, b time.Time) bool {
	return a.After(b) || a.Equal(b)
}

func BeforeOrEqual(a, b time.Time) bool {
	return a.Before(b) || a.Equal(b)
}

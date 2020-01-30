package analytics

import (
	"fmt"
	"os"
)

var disableAnalyticsEnvVars = []string{
	// Tilt-only analytics disabling
	"TILT_DISABLE_ANALYTICS",

	// https://consoledonottrack.com/
	"DO_NOT_TRACK",

	// We care about the number of humans using Tilt, not the number of CI machines
	"CI",
}

// If analytics is disabled, return a string representing a human-readable reason.
func IsAnalyticsDisabledFromEnv() (bool, string) {
	for _, key := range disableAnalyticsEnvVars {
		val := os.Getenv(key)
		if val != "" {
			return true, fmt.Sprintf("Environment variable %s=%s", key, val)
		}
	}
	return false, ""
}

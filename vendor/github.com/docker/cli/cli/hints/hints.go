package hints

import (
	"os"
	"strconv"
)

// Enabled returns whether cli hints are enabled or not
func Enabled() bool {
	if v := os.Getenv("DOCKER_CLI_HINTS"); v != "" {
		enabled, err := strconv.ParseBool(v)
		if err != nil {
			return true
		}
		return enabled
	}
	return true
}

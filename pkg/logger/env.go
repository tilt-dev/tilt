package logger

import (
	"context"
	"os"
	"strings"
)

// A set of environment variables to make sure that
// subprocesses log correctly.
func DefaultEnv(ctx context.Context) []string {
	supportsColor := Get(ctx).SupportsColor()
	env := os.Environ()
	hasLines := false
	hasColumns := false
	hasForceColor := false

	for _, e := range env {
		// LINES and COLUMNS are posix standards.
		// https://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap08.html
		hasLines = hasLines || strings.HasPrefix("LINES=", e)
		hasColumns = hasColumns || strings.HasPrefix("COLUMNS=", e)

		// FORCE_COLOR is common in nodejs https://github.com/tilt-dev/tilt/issues/3038
		hasForceColor = hasForceColor || strings.HasPrefix("FORCE_COLOR=", e)
	}

	if !hasLines {
		env = append(env, "LINES=24")
	}
	if !hasColumns {
		env = append(env, "COLUMNS=80")
	}
	if !hasForceColor && supportsColor {
		env = append(env, "FORCE_COLOR=1")
	}
	return env
}

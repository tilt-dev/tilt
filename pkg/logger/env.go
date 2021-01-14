package logger

import (
	"context"
	"os"
	"strings"
)

// DefaultEnv returns a set of strings in the form of "key=value"
// based on the current process' environment with additional entries
// to improve subprocess log output.
func DefaultEnv(ctx context.Context) []string {
	return PrepareEnv(Get(ctx), os.Environ())
}

// PrepareEnv returns a set of strings in the form of "key=value"
// based on a provided set of strings in the same format with additional
// entries to improve subprocess log output.
func PrepareEnv(l Logger, env []string) []string {
	supportsColor := l.SupportsColor()
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

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
	hasPythonUnbuffered := false

	for _, e := range env {
		// LINES and COLUMNS are posix standards.
		// https://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap08.html
		hasLines = hasLines || strings.HasPrefix(e, "LINES=")
		hasColumns = hasColumns || strings.HasPrefix(e, "COLUMNS=")

		// FORCE_COLOR is common in nodejs https://github.com/tilt-dev/tilt/issues/3038
		hasForceColor = hasForceColor || strings.HasPrefix(e, "FORCE_COLOR=")

		// PYTHONUNBUFFERED tells old Python versions not to buffer their output (< Python 3.7)
		// AIUI, older versions of Python buffer output aggressively when not connected to a TTY,
		// because they assume they're connected to a file and don't need realtime streaming.
		hasPythonUnbuffered = hasPythonUnbuffered || strings.HasPrefix(e, "PYTHONUNBUFFERED=")
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
	if !hasPythonUnbuffered {
		env = append(env, "PYTHONUNBUFFERED=1")
	}
	return env
}

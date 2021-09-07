package testutils

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// Setenv sets an environment variable for the duration of the test and restores the original value afterwards.
func Setenv(t testing.TB, key, value string) {
	t.Helper()
	captureAndRestoreEnv(t, key)
	require.NoError(t, os.Setenv(key, value))
}

// Unsetenv unsets an environment variable for the duration of the test and restores the original value afterwards.
func Unsetenv(t testing.TB, key string) {
	t.Helper()
	captureAndRestoreEnv(t, key)
	require.NoError(t, os.Unsetenv(key))
}

func captureAndRestoreEnv(t testing.TB, key string) {
	t.Helper()
	if curVal, ok := os.LookupEnv(key); ok {
		t.Cleanup(
			func() {
				if err := os.Setenv(key, curVal); err != nil {
					t.Errorf("Failed to restore env var %q: %v", key, err)
				}
			},
		)
	} else {
		t.Cleanup(
			func() {
				if err := os.Unsetenv(key); err != nil {
					t.Errorf("Failed to restore env var %q: %v", key, err)
				}
			},
		)
	}
}

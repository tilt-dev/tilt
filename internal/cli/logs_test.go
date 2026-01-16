package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvideLogSinceValidation(t *testing.T) {
	testCases := []struct {
		name        string
		flag        string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty is valid",
			flag:        "",
			expectError: false,
		},
		{
			name:        "positive duration is valid",
			flag:        "5m",
			expectError: false,
		},
		{
			name:        "negative duration is invalid",
			flag:        "-5m",
			expectError: true,
			errorMsg:    "must be positive",
		},
		{
			name:        "invalid format returns parse error",
			flag:        "notaduration",
			expectError: true,
			errorMsg:    "invalid duration",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Save and restore the global flag
			oldFlag := logSinceFlag
			defer func() { logSinceFlag = oldFlag }()

			logSinceFlag = tc.flag
			_, err := provideLogSince()

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestProvideLogTailValidation(t *testing.T) {
	testCases := []struct {
		name        string
		flag        int
		expectError bool
		errorMsg    string
	}{
		{
			name:        "-1 (no limit) is valid",
			flag:        -1,
			expectError: false,
		},
		{
			name:        "0 is valid",
			flag:        0,
			expectError: false,
		},
		{
			name:        "positive is valid",
			flag:        100,
			expectError: false,
		},
		{
			name:        "-2 is invalid",
			flag:        -2,
			expectError: true,
			errorMsg:    "must be -1 (no limit) or >= 0",
		},
		{
			name:        "-100 is invalid",
			flag:        -100,
			expectError: true,
			errorMsg:    "must be -1 (no limit) or >= 0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Save and restore the global flag
			oldFlag := logTailFlag
			defer func() { logTailFlag = oldFlag }()

			logTailFlag = tc.flag
			_, err := provideLogTail()

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

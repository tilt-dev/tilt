package stream

import (
	"errors"
	"os"
	"path/filepath"
)

func locateSocket() (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return "", errors.New("$HOME environment variable is undefined. unable to find socket for stream")
	}

	tiltDir := filepath.Join(home, ".tilt")
	return filepath.Join(tiltDir, "stream-socket"), nil
}

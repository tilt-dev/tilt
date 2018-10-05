package proto

import (
	"fmt"
	"os"

	"path/filepath"
)

func LocateSocket() (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return "", fmt.Errorf("internal/hud/proto/locate.go: can't find homedir")
	}

	tiltDir := filepath.Join(home, ".tilt")
	return filepath.Join(tiltDir, "socket"), nil
}

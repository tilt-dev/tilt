package web

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func StaticPath() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("Could not locate path to Tilt web static files")
	}

	// Double-check that the directory exists on disk and at least contains package.json
	dir := filepath.Dir(file)
	_, err := os.Stat(filepath.Join(dir, "package.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("Could not find Tilt web static files at path: %s", dir)
		}
		return "", fmt.Errorf("Could not find Tilt web static files: %v", err)
	}

	return dir, nil
}

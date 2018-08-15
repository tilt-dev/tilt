package fs

import (
	"path/filepath"
	"strings"
)

func PathIsChildOf(path string, parent string) bool {
	relPath, err := filepath.Rel(parent, path)
	if err != nil {
		return true
	}

	if relPath == "." {
		return true
	}

	if filepath.IsAbs(relPath) || strings.HasPrefix(relPath, "..") {
		return false
	}

	return true
}

package xdg

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func homeDir() string {
	homeEnv := "HOME"
	switch runtime.GOOS {
	case "windows":
		homeEnv = "USERPROFILE"
	case "plan9":
		homeEnv = "home"
	}

	if home := os.Getenv(homeEnv); home != "" {
		return home
	}

	switch runtime.GOOS {
	case "nacl":
		return "/"
	case "darwin":
		if runtime.GOARCH == "arm" || runtime.GOARCH == "arm64" {
			return "/"
		}
	}

	return ""
}

func expandPath(path, homeDir string) string {
	if path == "" || homeDir == "" {
		return path
	}
	if path[0] == '~' {
		return filepath.Join(homeDir, path[1:])
	}
	if strings.HasPrefix(path, "$HOME") {
		return filepath.Join(homeDir, path[5:])
	}

	return path
}

func createPath(name string, paths []string) (string, error) {
	var searchedPaths []string
	for _, p := range paths {
		path := filepath.Join(p, name)
		dir := filepath.Dir(path)

		if pathExists(dir) {
			return path, nil
		}
		if err := os.MkdirAll(dir, os.ModeDir|0700); err == nil {
			return path, nil
		}

		searchedPaths = append(searchedPaths, dir)
	}

	return "", fmt.Errorf("could not create any of the following paths: %s",
		strings.Join(searchedPaths, ", "))
}

func searchFile(name string, paths []string) (string, error) {
	var searchedPaths []string
	for _, p := range paths {
		path := filepath.Join(p, name)
		if pathExists(path) {
			return path, nil
		}

		searchedPaths = append(searchedPaths, filepath.Dir(path))
	}

	return "", fmt.Errorf("could not locate `%s` in any of the following paths: %s",
		filepath.Base(name), strings.Join(searchedPaths, ", "))
}

func xdgPath(name, defaultPath string) string {
	dir := expandPath(os.Getenv(name), Home)
	if dir != "" && filepath.IsAbs(dir) {
		return dir
	}

	return defaultPath
}

func xdgPaths(name string, defaultPaths ...string) []string {
	dirs := uniquePaths(filepath.SplitList(os.Getenv(name)))
	if len(dirs) != 0 {
		return dirs
	}

	return uniquePaths(defaultPaths)
}

func uniquePaths(paths []string) []string {
	var uniq []string
	registry := map[string]struct{}{}

	for _, p := range paths {
		dir := expandPath(p, Home)
		if dir == "" || !filepath.IsAbs(dir) {
			continue
		}
		if _, ok := registry[dir]; ok {
			continue
		}

		registry[dir] = struct{}{}
		uniq = append(uniq, dir)
	}

	return uniq
}

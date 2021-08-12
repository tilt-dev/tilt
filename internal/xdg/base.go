package xdg

import (
	"path/filepath"

	"github.com/adrg/xdg"
)

// Interface wrapper around
// https://github.com/adrg/xdg

type Base interface {
	CacheFile(relPath string) (string, error)
	ConfigFile(relPath string) (string, error)
	DataFile(relPath string) (string, error)
	StateFile(relPath string) (string, error)
	RuntimeFile(relPath string) (string, error)
}

const appName = "tilt-dev"

type tiltDevBase struct {
}

func NewTiltDevBase() Base {
	return tiltDevBase{}
}

func (tiltDevBase) CacheFile(relPath string) (string, error) {
	return xdg.CacheFile(filepath.Join(appName, relPath))
}
func (tiltDevBase) ConfigFile(relPath string) (string, error) {
	return xdg.ConfigFile(filepath.Join(appName, relPath))
}
func (tiltDevBase) DataFile(relPath string) (string, error) {
	return xdg.DataFile(filepath.Join(appName, relPath))
}
func (tiltDevBase) StateFile(relPath string) (string, error) {
	return xdg.StateFile(filepath.Join(appName, relPath))
}
func (tiltDevBase) RuntimeFile(relPath string) (string, error) {
	return xdg.RuntimeFile(filepath.Join(appName, relPath))
}

package extension

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
)

const extensionDirName = "tilt_modules"

type Store interface {
	// ModulePath is used to check if an extension exists before fetching it
	// Returns ErrNotExist if module doesn't exist
	ModulePath(ctx context.Context, moduleName string) (string, error)
	Write(ctx context.Context, contents ModuleContents) (string, error)
}

// TODO(dmiller): should this include an integrity hash?
type ModuleContents struct {
	Name              string
	TiltfileContents  string
	GitCommitHash     string
	ExtensionRegistry string
	TimeFetched       time.Time
}

type LocalStore struct {
	baseDir string
}

func NewLocalStore(baseDir string) *LocalStore {
	return &LocalStore{
		baseDir: filepath.Join(baseDir, extensionDirName),
	}
}

func (s *LocalStore) ModulePath(ctx context.Context, moduleName string) (string, error) {
	tiltfilePath := filepath.Join(s.baseDir, moduleName, "Tiltfile")

	_, err := os.Stat(tiltfilePath)
	if err != nil {
		return "", err
	}

	return tiltfilePath, nil
}

func (s *LocalStore) Write(ctx context.Context, contents ModuleContents) (string, error) {
	moduleDir := filepath.Join(s.baseDir, contents.Name)
	if err := os.MkdirAll(moduleDir, os.FileMode(0700)); err != nil {
		return "", errors.Wrapf(err, "couldn't create module directory %s at path %s", contents.Name, moduleDir)
	}

	tiltfilePath := filepath.Join(moduleDir, "Tiltfile")
	// TODO(dmiller): store hash, source, time fetched
	if err := ioutil.WriteFile(tiltfilePath, []byte(contents.TiltfileContents), os.FileMode(0600)); err != nil {
		return "", errors.Wrapf(err, "couldn't store module %s at path %s", contents.Name, tiltfilePath)
	}

	return tiltfilePath, nil
}

var _ Store = (*LocalStore)(nil)

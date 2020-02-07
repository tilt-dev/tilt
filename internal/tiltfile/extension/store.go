package extension

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/windmilleng/tilt/internal/tiltfile"
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
	Name             string
	TiltfileContents string
	GitCommitHash    string
	Source           string
	TimeFetched      time.Time
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
	tiltfilePath := filepath.Join(s.baseDir, moduleName, tiltfile.FileName)

	_, err := os.Stat(tiltfilePath)
	if err != nil {
		return "", err
	}

	return tiltfilePath, nil
}

func (s *LocalStore) Write(ctx context.Context, contents ModuleContents) (string, error) {
	moduleDir := filepath.Join(s.baseDir, contents.Name)
	if err := os.MkdirAll(moduleDir, os.FileMode(0700)); err != nil {
		return "", fmt.Errorf("couldn't store module %s: %v", contents.Name, err)
	}

	tiltfilePath := filepath.Join(moduleDir, tiltfile.FileName)
	// TODO(dmiller): store hash, source, time fetched
	if err := ioutil.WriteFile(tiltfilePath, []byte(contents.TiltfileContents), os.FileMode(0700)); err != nil {
		return "", fmt.Errorf("couldn't store module %s: %v", contents.Name, err)
	}

	return tiltfilePath, nil
}

var _ Store = (*LocalStore)(nil)

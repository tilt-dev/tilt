package xdg

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/afero"

	"github.com/tilt-dev/tilt/pkg/logger"
)

// A directory for the on-disk state of a single Tilt CLI run, named for its
// CLIWorkspaceID.
type CLIWorkspace struct {
	base        Base
	filesystem  afero.Fs
	workspaceID CLIWorkspaceID
	mu          sync.Mutex
	// Absolute path of this run's directory, resolved by the first OpenFile.
	// Empty until then.
	dir string
}

func NewCLIWorkspace(base Base, filesystem afero.Fs, workspaceID CLIWorkspaceID) *CLIWorkspace {
	return &CLIWorkspace{
		base:        base,
		filesystem:  filesystem,
		workspaceID: workspaceID,
	}
}

func ProvideCLIWorkspace(globalCtx context.Context, base Base, filesystem afero.Fs, workspaceID CLIWorkspaceID) (*CLIWorkspace, func()) {
	w := NewCLIWorkspace(base, filesystem, workspaceID)
	return w, func() {
		w.Cleanup(globalCtx)
	}
}

// Creates a file at relPath within this run's directory, along with any
// directories above it, and returns the file and its absolute path.
func (w *CLIWorkspace) OpenFile(ctx context.Context, relPath ...string) (string, afero.File, error) {
	dir, err := w.ensureDirExists(ctx, relPath)
	if err != nil {
		return "", nil, err
	}

	path := filepath.Join(append([]string{dir}, relPath...)...)
	f, err := w.filesystem.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return "", nil, err
	}
	return path, f, nil
}

// Resolves this run's directory, creating it on the first call and handing back
// that same directory on every call after.
func (w *CLIWorkspace) ensureDirExists(ctx context.Context, relPath []string) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.dir != "" {
		return w.dir, nil
	}

	dir, runtimeErr := w.touchUnder(w.base.RuntimeFile, relPath)
	if runtimeErr != nil {
		// No usable runtime dir. Normal in containers and CI, where
		// XDG_RUNTIME_DIR is unset and /run/user/<uid> can't be created. Note
		// where we ended up instead: the state root is never reclaimed for us.
		logger.Get(ctx).Debugf("no usable runtime dir (%v); falling back to the state dir", runtimeErr)

		var err error
		dir, err = w.touchUnder(w.base.StateFile, relPath)
		if err != nil {
			return "", err
		}
	}

	w.dir = dir
	return dir, nil
}

// Creates relPath under the given root -- both the file and the directories
// above it -- and returns this run's directory.
func (w *CLIWorkspace) touchUnder(root func(string) (string, error), relPath []string) (string, error) {
	path, err := root(filepath.Join(append([]string{string(w.workspaceID)}, relPath...)...))
	if err != nil {
		return "", err
	}

	f, err := w.filesystem.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	_ = f.Close()

	// Walk back up past relPath, to the directory named for the CLIWorkspaceID.
	dir := path
	for range relPath {
		dir = filepath.Dir(dir)
	}
	return dir, nil
}

// Deletes this run's directory.
func (w *CLIWorkspace) Cleanup(ctx context.Context) {
	w.mu.Lock()
	dir := w.dir
	w.dir = ""
	w.mu.Unlock()

	if dir == "" {
		return
	}

	if err := w.filesystem.RemoveAll(dir); err != nil {
		logger.Get(ctx).Debugf("removing CLI workspace dir %s: %v", dir, err)
	}
}

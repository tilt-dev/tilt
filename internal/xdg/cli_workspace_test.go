package xdg

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/pkg/logger"
)

func TestCleanupRemovesTheWholeWorkspace(t *testing.T) {
	f := newWorkspaceFixture(t, "tilt-default")

	nested := f.write("cluster", "default.yml")
	top := f.write("other.txt")

	f.workspace.Cleanup(f.ctx)
	f.assertGone(nested)
	f.assertGone(top)
	f.assertGone(filepath.Dir(nested))
}

func TestCleanupIsSafeWhenNothingWasWritten(t *testing.T) {
	f := newWorkspaceFixture(t, "tilt-default")
	f.workspace.Cleanup(f.ctx)
}

// The isolation that makes cleanup safe: one run's Cleanup can't reach another
// run's files. See tilt-dev/tilt#6826.
func TestCleanupLeavesOtherWorkspacesAlone(t *testing.T) {
	fs := afero.NewMemMapFs()
	base := NewFakeBase(t.TempDir(), fs)

	server := newWorkspaceFixtureWithFS(t, "tilt-default", base, fs)
	oneShot := newWorkspaceFixtureWithFS(t, "tilt-pid-1234", base, fs)

	serverPath := server.write("cluster", "default.yml")
	oneShotPath := oneShot.write("cluster", "default.yml")
	require.NotEqual(t, serverPath, oneShotPath)

	oneShot.workspace.Cleanup(oneShot.ctx)
	oneShot.assertGone(oneShotPath)
	server.assertExists(serverPath)
}

// Containers and CI often have no usable runtime dir: XDG_RUNTIME_DIR is unset
// and /run/user/<uid> can't be created.
func TestFallsBackWhenThereIsNoRuntimeDir(t *testing.T) {
	fs := afero.NewMemMapFs()
	base := &noRuntimeDirBase{Base: NewFakeBase(t.TempDir(), fs)}
	f := newWorkspaceFixtureWithFS(t, "tilt-pid-1234", base, fs)

	path := f.write("cluster", "default.yml")
	assert.Contains(t, path, filepath.Join("state", "tilt-pid-1234", "cluster"))
}

// Creating the directory isn't proof that we can write into it: it may be
// read-only, or hold a file of the same name that we don't own.
func TestFallsBackWhenTheRuntimeDirIsUnwritable(t *testing.T) {
	root := t.TempDir()
	fs := unwritableFs{Fs: afero.NewMemMapFs(), prefix: filepath.Join(root, "runtime")}
	f := newWorkspaceFixtureWithFS(t, "tilt-pid-1234", NewFakeBase(root, fs), fs)

	path := f.write("cluster", "default.yml")
	assert.Contains(t, path, filepath.Join("state", "tilt-pid-1234", "cluster"))
}

// Once we've fallen back, every later write goes to the same root. Straddling
// both would leave copies behind in whichever one we stopped using.
func TestTheRootIsResolvedOnce(t *testing.T) {
	fs := afero.NewMemMapFs()
	base := &noRuntimeDirBase{Base: NewFakeBase(t.TempDir(), fs)}
	f := newWorkspaceFixtureWithFS(t, "tilt-pid-1234", base, fs)

	first := f.write("cluster", "default.yml")

	// The runtime dir becomes usable partway through the run's life.
	base.working = true

	second := f.write("cluster", "staging.yml")
	assert.Equal(t, filepath.Dir(first), filepath.Dir(second))
}

// An xdg.Base with no usable runtime dir, until `working` is set.
type noRuntimeDirBase struct {
	Base
	working bool
}

func (b *noRuntimeDirBase) RuntimeFile(relPath string) (string, error) {
	if !b.working {
		return "", fmt.Errorf("no runtime dir")
	}
	return b.Base.RuntimeFile(relPath)
}

// An afero.Fs that creates directories happily but refuses to open files under
// `prefix`.
type unwritableFs struct {
	afero.Fs
	prefix string
}

func (fs unwritableFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if strings.HasPrefix(name, fs.prefix) {
		return nil, fmt.Errorf("permission denied")
	}
	return fs.Fs.OpenFile(name, flag, perm)
}

type workspaceFixture struct {
	t         *testing.T
	ctx       context.Context
	fs        afero.Fs
	workspace *CLIWorkspace
}

func newWorkspaceFixture(t *testing.T, workspaceID CLIWorkspaceID) *workspaceFixture {
	fs := afero.NewMemMapFs()
	return newWorkspaceFixtureWithFS(t, workspaceID, NewFakeBase(t.TempDir(), fs), fs)
}

func newWorkspaceFixtureWithFS(t *testing.T, workspaceID CLIWorkspaceID, base Base, fs afero.Fs) *workspaceFixture {
	return &workspaceFixture{
		t:         t,
		ctx:       logger.WithLogger(context.Background(), logger.NewLogger(logger.DebugLvl, io.Discard)),
		fs:        fs,
		workspace: NewCLIWorkspace(base, fs, workspaceID),
	}
}

func (f *workspaceFixture) write(relPath ...string) string {
	f.t.Helper()
	path, file, err := f.workspace.OpenFile(f.ctx, relPath...)
	require.NoError(f.t, err)
	require.NoError(f.t, file.Close())
	return path
}

func (f *workspaceFixture) assertExists(path string) {
	f.t.Helper()
	exists, err := afero.Exists(f.fs, path)
	require.NoError(f.t, err)
	assert.True(f.t, exists, "expected %s to exist", path)
}

func (f *workspaceFixture) assertGone(path string) {
	f.t.Helper()
	exists, err := afero.Exists(f.fs, path)
	require.NoError(f.t, err)
	assert.False(f.t, exists, "expected %s to be cleaned up", path)
}

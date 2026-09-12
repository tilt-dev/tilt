package kubeconfig

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/tilt-dev/tilt/internal/xdg"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
)

func TestWritesAFrozenKubeConfig(t *testing.T) {
	f := newWriterFixture(t)

	path := f.write(v1alpha1.ClusterNameDefault)
	assert.Equal(t, filepath.Join(clusterDir, "default.yml"),
		filepath.Join(filepath.Base(filepath.Dir(path)), filepath.Base(path)))

	// The point of freezing: the config names one context, and carries its
	// credentials inline rather than by reference.
	config := f.read(path)
	assert.Equal(t, "default", config.CurrentContext)
	assert.Len(t, config.Contexts, 1)
	assert.Equal(t, "token", config.AuthInfos["default"].Token)
}

func TestEachClusterGetsItsOwnFile(t *testing.T) {
	f := newWriterFixture(t)

	defaultPath := f.write(v1alpha1.ClusterNameDefault)
	stagingPath := f.write("staging")

	assert.NotEqual(t, defaultPath, stagingPath)
	assert.Equal(t, filepath.Dir(defaultPath), filepath.Dir(stagingPath))
}

// The configs are written into the CLI run's workspace, so the run's cleanup
// takes them with it.
func TestWorkspaceCleanupRemovesTheConfigs(t *testing.T) {
	f := newWriterFixture(t)

	path := f.write(v1alpha1.ClusterNameDefault)

	f.workspace.Cleanup(f.ctx)

	exists, err := afero.Exists(f.fs, path)
	require.NoError(t, err)
	assert.False(t, exists, "expected %s to be cleaned up", path)
}

func validAPIConfig() *api.Config {
	return &api.Config{
		CurrentContext: "default",
		Contexts: map[string]*api.Context{
			"default": {Cluster: "default", AuthInfo: "default"},
			"unused":  {Cluster: "unused", AuthInfo: "unused"},
		},
		Clusters: map[string]*api.Cluster{
			"default": {Server: "https://localhost:6443"},
			"unused":  {Server: "https://localhost:7443"},
		},
		AuthInfos: map[string]*api.AuthInfo{
			"default": {Token: "token"},
			"unused":  {Token: "unused-token"},
		},
	}
}

type writerFixture struct {
	t         *testing.T
	ctx       context.Context
	fs        afero.Fs
	workspace *xdg.CLIWorkspace
	writer    *Writer
}

func newWriterFixture(t *testing.T) *writerFixture {
	fs := afero.NewMemMapFs()
	workspace := xdg.NewCLIWorkspace(xdg.NewFakeBase(t.TempDir(), fs), fs, "tilt-default")
	return &writerFixture{
		t:         t,
		ctx:       logger.WithLogger(context.Background(), logger.NewLogger(logger.DebugLvl, io.Discard)),
		fs:        fs,
		workspace: workspace,
		writer:    NewWriter(workspace),
	}
}

func (f *writerFixture) write(clusterName string) string {
	f.t.Helper()
	path, err := f.writer.WriteFrozenKubeConfig(f.ctx,
		types.NamespacedName{Name: clusterName},
		validAPIConfig())
	require.NoError(f.t, err)
	return path
}

func (f *writerFixture) read(path string) *api.Config {
	f.t.Helper()
	contents, err := afero.ReadFile(f.fs, path)
	require.NoError(f.t, err)
	config, err := clientcmd.Load(contents)
	require.NoError(f.t, err)
	return config
}

package tiltfile

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/docker/docker/builder/dockerignore"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/equality"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestFileWatch_basic(t *testing.T) {
	f := newFWFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	f.SetManifestDCTarget(target)

	f.RequireFileWatchSpecEqual(target.ID(), v1alpha1.FileWatchSpec{WatchedPaths: []string{"."}})
}

func TestFileWatch_localRepo(t *testing.T) {
	f := newFWFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".").
		WithRepos([]model.LocalGitRepo{model.LocalGitRepo{LocalPath: "."}})
	f.SetManifestDCTarget(target)

	f.RequireFileWatchSpecEqual(target.ID(), v1alpha1.FileWatchSpec{
		WatchedPaths: []string{"."},
		Ignores: []v1alpha1.IgnoreDef{
			{BasePath: filepath.Join(".", ".git")},
		},
	})
}

func TestFileWatch_disabledOnCIMode(t *testing.T) {
	f := newFWFixture(t)
	defer f.TearDown()

	f.inputs.EngineMode = store.EngineModeCI

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	f.SetManifestDCTarget(target)
	m := model.Manifest{Name: "foo"}.WithDeployTarget(target)
	f.SetManifest(m)

	actualSet := ToFileWatchObjects(f.inputs, make(map[string]*v1alpha1.DisableSource))
	assert.Empty(t, actualSet)
}

func TestFileWatch_IgnoredLocalDirectories(t *testing.T) {
	f := newFWFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithIgnoredLocalDirectories([]string{"bar"}).
		WithBuildPath(".")
	f.SetManifestDCTarget(target)

	f.RequireFileWatchSpecEqual(target.ID(), v1alpha1.FileWatchSpec{
		WatchedPaths: []string{"."},
		Ignores: []v1alpha1.IgnoreDef{
			{BasePath: "bar"},
		},
	})
}

func TestFileWatch_Dockerignore(t *testing.T) {
	f := newFWFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithDockerignores([]model.Dockerignore{{LocalPath: ".", Patterns: []string{"bar"}}}).
		WithBuildPath(".")
	f.SetManifestDCTarget(target)

	f.RequireFileWatchSpecEqual(target.ID(), v1alpha1.FileWatchSpec{
		WatchedPaths: []string{"."},
		Ignores: []v1alpha1.IgnoreDef{
			{BasePath: ".", Patterns: []string{"bar"}},
		},
	})
}

func TestFileWatch_IgnoreOutputsImageRefs(t *testing.T) {
	f := newFWFixture(t)
	defer f.TearDown()

	target := model.MustNewImageTarget(container.MustParseSelector("img")).
		WithBuildDetails(model.CustomBuild{
			Deps:              []string{f.Path()},
			OutputsImageRefTo: f.JoinPath("ref.txt"),
		})

	m := manifestbuilder.New(f, "sancho").
		WithK8sYAML(testyaml.SanchoYAML).
		WithImageTarget(target).
		Build()
	f.SetManifest(m)

	f.RequireFileWatchSpecEqual(target.ID(), v1alpha1.FileWatchSpec{
		WatchedPaths: []string{f.Path()},
		Ignores: []v1alpha1.IgnoreDef{
			{BasePath: f.Path(), Patterns: []string{"ref.txt"}},
		},
	})
}

func TestFileWatch_WatchesReappliedOnDockerComposeSyncChange(t *testing.T) {
	f := newFWFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	f.SetManifestDCTarget(target.WithIgnoredLocalDirectories([]string{"bar"}))
	f.RequireFileWatchSpecEqual(target.ID(), v1alpha1.FileWatchSpec{
		WatchedPaths: []string{"."},
		Ignores: []v1alpha1.IgnoreDef{
			{BasePath: "bar"},
		},
	})

	f.SetManifestDCTarget(target)
	f.RequireFileWatchSpecEqual(target.ID(), v1alpha1.FileWatchSpec{WatchedPaths: []string{"."}})
}

func TestFileWatch_WatchesReappliedOnDockerIgnoreChange(t *testing.T) {
	f := newFWFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	f.SetManifestDCTarget(target.WithDockerignores([]model.Dockerignore{{LocalPath: ".", Patterns: []string{"bar"}}}))
	f.RequireFileWatchSpecEqual(target.ID(), v1alpha1.FileWatchSpec{
		WatchedPaths: []string{"."},
		Ignores: []v1alpha1.IgnoreDef{
			{BasePath: ".", Patterns: []string{"bar"}},
		},
	})

	f.SetManifestDCTarget(target)
	f.RequireFileWatchSpecEqual(target.ID(), v1alpha1.FileWatchSpec{WatchedPaths: []string{"."}})
}

func TestFileWatch_ConfigFiles(t *testing.T) {
	f := newFWFixture(t)
	defer f.TearDown()

	f.SetTiltIgnoreContents("**/foo")
	f.inputs.ConfigFiles = append(f.inputs.ConfigFiles, "path_to_watch", "stop")

	id := model.TargetID{Type: model.TargetTypeConfigs, Name: model.TargetName(model.MainTiltfileManifestName)}
	f.RequireFileWatchSpecEqual(id, v1alpha1.FileWatchSpec{
		WatchedPaths: []string{"path_to_watch", "stop"},
		Ignores: []v1alpha1.IgnoreDef{
			{BasePath: f.Path(), Patterns: []string{"**/foo"}},
		},
	})
}

func TestFileWatch_IgnoreTiltIgnore(t *testing.T) {
	f := newFWFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	f.SetManifestDCTarget(target)
	f.SetTiltIgnoreContents("**/foo")
	f.RequireFileWatchSpecEqual(target.ID(), v1alpha1.FileWatchSpec{
		WatchedPaths: []string{"."},
		Ignores: []v1alpha1.IgnoreDef{
			{BasePath: f.Path(), Patterns: []string{"**/foo"}},
		},
	})
}

func TestFileWatch_IgnoreWatchSettings(t *testing.T) {
	f := newFWFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	f.SetManifestDCTarget(target)

	f.inputs.WatchSettings.Ignores = append(f.inputs.WatchSettings.Ignores, model.Dockerignore{
		LocalPath: f.Path(),
		Patterns:  []string{"**/foo"},
	})

	f.RequireFileWatchSpecEqual(target.ID(), v1alpha1.FileWatchSpec{
		WatchedPaths: []string{"."},
		Ignores: []v1alpha1.IgnoreDef{
			{BasePath: f.Path(), Patterns: []string{"**/foo"}},
		},
	})
}

func TestFileWatch_PickUpTiltIgnoreChanges(t *testing.T) {
	f := newFWFixture(t)
	defer f.TearDown()

	target := model.DockerComposeTarget{Name: "foo"}.
		WithBuildPath(".")
	f.SetManifestDCTarget(target)
	f.SetTiltIgnoreContents("**/foo")
	f.RequireFileWatchSpecEqual(target.ID(), v1alpha1.FileWatchSpec{
		WatchedPaths: []string{"."},
		Ignores: []v1alpha1.IgnoreDef{
			{BasePath: f.Path(), Patterns: []string{"**/foo"}},
		},
	})

	f.SetTiltIgnoreContents("**foo\n!bar/baz/foo")
	f.RequireFileWatchSpecEqual(target.ID(), v1alpha1.FileWatchSpec{
		WatchedPaths: []string{"."},
		Ignores: []v1alpha1.IgnoreDef{
			{BasePath: f.Path(), Patterns: []string{"**foo", "!bar/baz/foo"}},
		},
	})
}

type fwFixture struct {
	t   testing.TB
	ctx context.Context
	cli ctrlclient.Client
	*tempdir.TempDirFixture
	inputs WatchInputs
}

func newFWFixture(t *testing.T) *fwFixture {
	cli := fake.NewFakeTiltClient()

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)

	tmpdir := tempdir.NewTempDirFixture(t)
	tmpdir.Chdir()

	f := &fwFixture{
		t:              t,
		ctx:            ctx,
		cli:            cli,
		TempDirFixture: tmpdir,
		inputs:         WatchInputs{TiltfileManifestName: model.MainTiltfileManifestName},
	}

	t.Cleanup(func() {
		tmpdir.TearDown()
		cancel()
	})

	return f
}

type fileWatchDiffer struct {
	expected v1alpha1.FileWatchSpec
	actual   *v1alpha1.FileWatchSpec
}

func (f fileWatchDiffer) String() string {
	return cmp.Diff(&f.expected, f.actual)
}

func (f *fwFixture) RequireFileWatchSpecEqual(targetID model.TargetID, spec v1alpha1.FileWatchSpec) {
	f.t.Helper()

	actualSet := ToFileWatchObjects(f.inputs, make(map[string]*v1alpha1.DisableSource))
	actual, ok := actualSet[apis.SanitizeName(targetID.String())]
	require.True(f.T(), ok, "No filewatch found for %s", targetID)
	fwd := &fileWatchDiffer{expected: spec}
	require.True(f.T(), equality.Semantic.DeepEqual(actual.GetSpec(), spec), "FileWatch spec was not equal: %v", fwd)
}

func (f *fwFixture) SetManifestDCTarget(target model.DockerComposeTarget) {
	m := model.Manifest{Name: "foo"}.WithDeployTarget(target)
	f.SetManifest(m)
}

func (f *fwFixture) SetManifest(m model.Manifest) {
	for i, original := range f.inputs.Manifests {
		if original.Name == m.Name {
			f.inputs.Manifests[i] = m
			return
		}
	}
	f.inputs.Manifests = append(f.inputs.Manifests, m)
}

func (f *fwFixture) SetTiltIgnoreContents(s string) {
	patterns, err := dockerignore.ReadAll(strings.NewReader(s))
	assert.NoError(f.T(), err)
	f.inputs.Tiltignore = model.Dockerignore{
		LocalPath: f.Path(),
		Patterns:  patterns,
	}
}

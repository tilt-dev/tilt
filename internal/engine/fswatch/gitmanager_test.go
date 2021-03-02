package fswatch

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/watch"

	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestGitManagerAdd(t *testing.T) {
	f := newGMFixture(t)
	defer f.TearDown()

	head := f.JoinPath(".git", "HEAD")
	f.WriteFile(head, "ref: refs/heads/nicks/branch")
	f.UpsertManifestTarget("fe", model.LocalGitRepo{LocalPath: f.Path()})

	f.gm.OnChange(f.ctx, f.store)
	assert.Equal(t, "ref: refs/heads/nicks/branch",
		f.NextGitBranchStatusAction().Head)

	f.store.ClearActions()
	f.WriteFile(head, "ref: refs/heads/nicks/branch2")
	f.fakeMultiWatcher.Events <- watch.NewFileEvent(head)
	assert.Equal(t, "ref: refs/heads/nicks/branch2",
		f.NextGitBranchStatusAction().Head)
}

func TestGitManagerRemove(t *testing.T) {
	f := newGMFixture(t)
	defer f.TearDown()

	head := f.JoinPath(".git", "HEAD")
	f.WriteFile(head, "ref: refs/heads/nicks/branch")
	f.UpsertManifestTarget("fe", model.LocalGitRepo{LocalPath: f.Path()})
	f.gm.OnChange(f.ctx, f.store)
	assert.Equal(t, "ref: refs/heads/nicks/branch",
		f.NextGitBranchStatusAction().Head)
	f.store.ClearActions()

	f.WriteFile(head, "ref: refs/heads/nicks/branch2")
	f.fakeMultiWatcher.Events <- watch.NewFileEvent(head)
	assert.Equal(t, "ref: refs/heads/nicks/branch2",
		f.NextGitBranchStatusAction().Head)

	f.UpsertManifestTarget("fe")
	f.gm.OnChange(f.ctx, f.store)
	f.store.ClearActions()

	f.WriteFile(head, "ref: refs/heads/nicks/branch3")
	f.fakeMultiWatcher.Events <- watch.NewFileEvent(head)
	store.AssertNoActionOfType(t,
		reflect.TypeOf(GitBranchStatusAction{}), f.store.Actions)
}

type gmFixture struct {
	ctx              context.Context
	cancel           func()
	store            *store.TestingStore
	gm               *GitManager
	fakeMultiWatcher *watch.FakeMultiWatcher
	*tempdir.TempDirFixture
}

func newGMFixture(t *testing.T) *gmFixture {
	st := store.NewTestingStore()
	fakeMultiWatcher := watch.NewFakeMultiWatcher()
	gm := NewGitManager(fakeMultiWatcher.NewSub)

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)

	f := tempdir.NewTempDirFixture(t)
	f.Chdir()

	return &gmFixture{
		ctx:              ctx,
		cancel:           cancel,
		store:            st,
		gm:               gm,
		fakeMultiWatcher: fakeMultiWatcher,
		TempDirFixture:   f,
	}
}

func (f *gmFixture) TearDown() {
	f.TempDirFixture.TearDown()
	f.cancel()
	f.store.AssertNoErrorActions(f.T())
}

func (f *gmFixture) NextGitBranchStatusAction() GitBranchStatusAction {
	return f.store.WaitForAction(f.T(), reflect.TypeOf(GitBranchStatusAction{})).(GitBranchStatusAction)
}

func (f *gmFixture) UpsertManifestTarget(name model.ManifestName, repos ...model.LocalGitRepo) {
	f.store.WithState(func(state *store.EngineState) {
		m := manifestbuilder.New(f, name).
			WithK8sYAML(testyaml.SanchoYAML).
			WithImageTarget(
				model.ImageTarget{}.
					WithRepos(repos)).
			Build()
		state.UpsertManifestTarget(store.NewManifestTarget(m))
	})
}

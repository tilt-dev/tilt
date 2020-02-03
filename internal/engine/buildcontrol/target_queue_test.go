package buildcontrol

import (
	"context"
	"fmt"
	"testing"

	"github.com/docker/distribution/reference"
	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/testutils"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestTargetQueue_Simple(t *testing.T) {
	f := newTargetQueueFixture(t)

	t1 := model.MustNewImageTarget(container.MustParseSelector("vigoda"))
	s1 := store.BuildState{}

	targets := []model.ImageTarget{t1}
	buildStateSet := store.BuildStateSet{
		t1.ID(): s1,
	}

	f.run(targets, buildStateSet)

	expectedCalls := map[model.TargetID]fakeBuildHandlerCall{
		t1.ID(): newFakeBuildHandlerCall(t1, s1, 1, []store.BuildResult{}),
	}
	assert.Equal(t, expectedCalls, f.handler.calls)
}

func TestTargetQueue_DepsBuilt(t *testing.T) {
	f := newTargetQueueFixture(t)

	fooTarget := model.MustNewImageTarget(container.MustParseSelector("foo"))
	s1 := store.BuildState{LastSuccessfulResult: store.NewImageBuildResultSingleRef(fooTarget.ID(), container.MustParseNamedTagged("foo:1234"))}
	barTarget := model.MustNewImageTarget(container.MustParseSelector("bar")).WithDependencyIDs([]model.TargetID{fooTarget.ID()})
	s2 := store.BuildState{}

	targets := []model.ImageTarget{fooTarget, barTarget}
	buildStateSet := store.BuildStateSet{
		fooTarget.ID(): s1,
		barTarget.ID(): s2,
	}

	f.run(targets, buildStateSet)

	barCall := newFakeBuildHandlerCall(barTarget, s2, 1, []store.BuildResult{
		store.NewImageBuildResultSingleRef(fooTarget.ID(), store.LocalImageRefFromBuildResult(s1.LastSuccessfulResult)),
	})

	// foo has a valid last result, so only bar gets rebuilt
	expectedCalls := map[model.TargetID]fakeBuildHandlerCall{
		barTarget.ID(): barCall,
	}
	assert.Equal(t, expectedCalls, f.handler.calls)
}

func TestTargetQueue_DepsUnbuilt(t *testing.T) {
	f := newTargetQueueFixture(t)

	fooTarget := model.MustNewImageTarget(container.MustParseSelector("foo"))
	s1 := store.BuildState{}
	barTarget := model.MustNewImageTarget(container.MustParseSelector("bar")).WithDependencyIDs([]model.TargetID{fooTarget.ID()})
	var s2 = store.BuildState{LastSuccessfulResult: store.NewImageBuildResultSingleRef(
		barTarget.ID(),
		container.MustParseNamedTagged("bar:54321"),
	)}
	targets := []model.ImageTarget{fooTarget, barTarget}
	buildStateSet := store.BuildStateSet{
		fooTarget.ID(): s1,
		barTarget.ID(): s2,
	}

	f.run(targets, buildStateSet)

	fooCall := newFakeBuildHandlerCall(fooTarget, s1, 1, []store.BuildResult{})
	// bar's dep is dirty, so bar should not get its old state
	barCall := newFakeBuildHandlerCall(barTarget, store.BuildState{}, 2, []store.BuildResult{fooCall.result})

	expectedCalls := map[model.TargetID]fakeBuildHandlerCall{
		fooTarget.ID(): fooCall,
		barTarget.ID(): barCall,
	}
	assert.Equal(t, expectedCalls, f.handler.calls)
}

func TestTargetQueue_IncrementalBuild(t *testing.T) {
	f := newTargetQueueFixture(t)

	fooTarget := model.MustNewImageTarget(container.MustParseSelector("foo"))
	s1 := store.BuildState{
		LastSuccessfulResult: store.NewImageBuildResultSingleRef(
			fooTarget.ID(),
			container.MustParseNamedTagged("foo:1234"),
		),
		FilesChangedSet: map[string]bool{"hello.txt": true},
	}

	targets := []model.ImageTarget{fooTarget}
	buildStateSet := store.BuildStateSet{fooTarget.ID(): s1}

	f.run(targets, buildStateSet)

	fooCall := newFakeBuildHandlerCall(fooTarget, s1, 1, []store.BuildResult{})

	expectedCalls := map[model.TargetID]fakeBuildHandlerCall{
		fooTarget.ID(): fooCall,
	}
	assert.Equal(t, expectedCalls, f.handler.calls)
}

func TestTargetQueue_CachedBuild(t *testing.T) {
	f := newTargetQueueFixture(t)

	fooTarget := model.MustNewImageTarget(container.MustParseSelector("foo"))
	s1 := store.BuildState{
		LastSuccessfulResult: store.NewImageBuildResultSingleRef(
			fooTarget.ID(),
			container.MustParseNamedTagged("foo:1234"),
		),
	}

	targets := []model.ImageTarget{fooTarget}
	buildStateSet := store.BuildStateSet{fooTarget.ID(): s1}

	f.run(targets, buildStateSet)

	// last result is still valid, so handler doesn't get called at all
	expectedCalls := map[model.TargetID]fakeBuildHandlerCall{}
	assert.Equal(t, expectedCalls, f.handler.calls)
}

func TestTargetQueue_DepsBuiltButReaped(t *testing.T) {
	f := newTargetQueueFixture(t)

	fooTarget := model.MustNewImageTarget(container.MustParseSelector("foo"))
	s1 := store.BuildState{LastSuccessfulResult: store.NewImageBuildResultSingleRef(fooTarget.ID(), container.MustParseNamedTagged("foo:1234"))}
	barTarget := model.MustNewImageTarget(container.MustParseSelector("bar")).WithDependencyIDs([]model.TargetID{fooTarget.ID()})
	s2 := store.BuildState{}

	targets := []model.ImageTarget{fooTarget, barTarget}
	buildStateSet := store.BuildStateSet{
		fooTarget.ID(): s1,
		barTarget.ID(): s2,
	}

	f.setMissingImage(store.LocalImageRefFromBuildResult(s1.LastSuccessfulResult))

	f.run(targets, buildStateSet)

	fooCall := newFakeBuildHandlerCall(fooTarget, s1, 1, []store.BuildResult{})
	barCall := newFakeBuildHandlerCall(barTarget, s2, 2, []store.BuildResult{
		store.NewImageBuildResultSingleRef(fooTarget.ID(), store.LocalImageRefFromBuildResult(fooCall.result)),
	})

	// foo has a valid last result, but its image is missing, so we have to rebuild it and its deps
	expectedCalls := map[model.TargetID]fakeBuildHandlerCall{
		fooTarget.ID(): fooCall,
		barTarget.ID(): barCall,
	}
	assert.Equal(t, expectedCalls, f.handler.calls)
}

func newFakeBuildHandlerCall(target model.ImageTarget, state store.BuildState, num int, depResults []store.BuildResult) fakeBuildHandlerCall {
	return fakeBuildHandlerCall{
		target: target,
		state:  state,
		result: store.NewImageBuildResultSingleRef(
			target.ID(),
			container.MustParseNamedTagged(fmt.Sprintf("%s:%d", target.Refs.ConfigurationRef.String(), num)),
		),
		depResults: depResults,
	}
}

type fakeBuildHandlerCall struct {
	target     model.TargetSpec
	state      store.BuildState
	depResults []store.BuildResult
	result     store.BuildResult
}

type fakeBuildHandler struct {
	buildNum int
	calls    map[model.TargetID]fakeBuildHandlerCall
}

func newFakeBuildHandler() *fakeBuildHandler {
	return &fakeBuildHandler{
		calls: make(map[model.TargetID]fakeBuildHandlerCall),
	}
}

func (fbh *fakeBuildHandler) handle(target model.TargetSpec, state store.BuildState, depResults []store.BuildResult) (store.BuildResult, error) {
	iTarget := target.(model.ImageTarget)
	fbh.buildNum++
	namedTagged := container.MustParseNamedTagged(fmt.Sprintf("%s:%d", iTarget.Refs.ConfigurationRef, fbh.buildNum))
	result := store.NewImageBuildResultSingleRef(target.ID(), namedTagged)
	fbh.calls[target.ID()] = fakeBuildHandlerCall{target, state, depResults, result}
	return result, nil
}

type targetQueueFixture struct {
	t             *testing.T
	ctx           context.Context
	handler       *fakeBuildHandler
	missingImages []reference.NamedTagged
}

func newTargetQueueFixture(t *testing.T) *targetQueueFixture {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	return &targetQueueFixture{
		t:       t,
		ctx:     ctx,
		handler: newFakeBuildHandler(),
	}
}

func (f *targetQueueFixture) imageExists(ctx context.Context, namedTagged reference.NamedTagged) (b bool, e error) {
	for _, ref := range f.missingImages {
		if ref == namedTagged {
			return false, nil
		}
	}
	return true, nil
}

func (f *targetQueueFixture) setMissingImage(namedTagged reference.NamedTagged) {
	f.missingImages = append(f.missingImages, namedTagged)
}

func (f *targetQueueFixture) run(targets []model.ImageTarget, buildStateSet store.BuildStateSet) {
	tq, err := NewImageTargetQueue(f.ctx, targets, buildStateSet, f.imageExists)
	if err != nil {
		f.t.Fatal(err)
	}

	err = tq.RunBuilds(f.handler.handle)
	if err != nil {
		f.t.Fatal(err)
	}
}

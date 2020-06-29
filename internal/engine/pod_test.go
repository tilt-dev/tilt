package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/engine/k8swatch"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/podbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestPodDeleteAction(t *testing.T) {
	f := newReducerFixture(t)
	defer f.TearDown()

	ms, _ := f.state.ManifestState("sancho")
	m, _ := f.state.Manifest("sancho")
	hash := k8s.PodTemplateSpecHash("ptsh")
	pod := podbuilder.New(f.T(), m).WithTemplateSpecHash(hash).Build()
	runtime := ms.K8sRuntimeState()
	runtime.DeployedPodTemplateSpecHashSet.Add(hash)

	assert.Equal(t, 0, len(ms.K8sRuntimeState().Pods))

	handlePodChangeAction(f.ctx, f.state, k8swatch.PodChangeAction{
		Pod:          pod,
		ManifestName: m.Name,
	})

	assert.Equal(t, 1, len(ms.K8sRuntimeState().Pods))

	handlePodDeleteAction(f.ctx, f.state, k8swatch.PodDeleteAction{
		PodID: k8s.PodIDFromPod(pod),
	})
	assert.Equal(t, 0, len(ms.K8sRuntimeState().Pods))
}

// A simple fixture for testing reducers, independently of a store.
type reducerFixture struct {
	*tempdir.TempDirFixture
	ctx   context.Context
	state *store.EngineState
}

func newReducerFixture(t *testing.T) *reducerFixture {
	f := tempdir.NewTempDirFixture(t)
	state := store.NewState()

	iTarget := NewSanchoLiveUpdateImageTarget(f)
	manifest := manifestbuilder.New(f, model.ManifestName("sancho")).
		WithK8sYAML(SanchoYAML).
		WithImageTarget(iTarget).
		Build()
	state.UpsertManifestTarget(store.NewManifestTarget(manifest))
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()

	return &reducerFixture{
		TempDirFixture: f,
		ctx:            ctx,
		state:          state,
	}
}

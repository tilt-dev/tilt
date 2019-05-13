package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/wmclient/pkg/analytics"
)

func TestNeedsNudgeK8sYaml(t *testing.T) {
	st, _ := store.NewStoreForTesting()
	state := st.LockMutableStateForTesting()
	defer st.UnlockMutableState()

	m := k8s.NewK8sOnlyManifestForTesting("yamlyaml", nil)
	targ := store.NewManifestTarget(m)
	targ.State = &store.ManifestState{LastSuccessfulDeployTime: time.Now()}
	maybeSetNeedsNudge(targ, state)
	assert.False(t, state.NeedsAnalyticsNudge,
		"manifest is k8s_yaml, expected needsNudge = false")
}

func TestNeedsNudgeRedManifest(t *testing.T) {
	st, _ := store.NewStoreForTesting()
	state := st.LockMutableStateForTesting()
	defer st.UnlockMutableState()

	m := model.Manifest{Name: "server"}
	targ := store.NewManifestTarget(m)
	maybeSetNeedsNudge(targ, state)
	assert.False(t, state.NeedsAnalyticsNudge,
		"manifest has never had successful build, expected needsNudge = false")
}

func TestNeedsNudgeGreenManifest(t *testing.T) {
	st, _ := store.NewStoreForTesting()
	state := st.LockMutableStateForTesting()
	defer st.UnlockMutableState()

	m := model.Manifest{Name: "server"}
	targ := store.NewManifestTarget(m)
	targ.State = &store.ManifestState{LastSuccessfulDeployTime: time.Now()}
	maybeSetNeedsNudge(targ, state)
	assert.True(t, state.NeedsAnalyticsNudge,
		"manifest HAS had had successful build, expected needsNudge = true")
}

func TestNeedsNudgeAlreadyNeedsNudge(t *testing.T) {
	st, _ := store.NewStoreForTesting()
	state := st.LockMutableStateForTesting()
	defer st.UnlockMutableState()

	state.NeedsAnalyticsNudge = true
	m := model.Manifest{Name: "server"}
	targ := store.NewManifestTarget(m)
	maybeSetNeedsNudge(targ, state)
	assert.True(t, state.NeedsAnalyticsNudge,
		"needsNudge already set, expected no change")
}

func TestNeedsNudgeAlreadyOpted(t *testing.T) {
	st, _ := store.NewStoreForTesting()
	state := st.LockMutableStateForTesting()
	defer st.UnlockMutableState()

	state.AnalyticsOpt = analytics.OptIn
	m := model.Manifest{Name: "server"}
	targ := store.NewManifestTarget(m)
	targ.State = &store.ManifestState{LastSuccessfulDeployTime: time.Now()}
	maybeSetNeedsNudge(targ, state)
	assert.False(t, state.NeedsAnalyticsNudge,
		"user already opted in, expected needsNudge = false")
}

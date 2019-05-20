package webview

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/wmclient/pkg/analytics"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

func TestNeedsNudgeK8sYaml(t *testing.T) {
	state := store.NewState()

	m := k8s.NewK8sOnlyManifestForTesting("yamlyaml", nil)
	targ := store.NewManifestTarget(m)
	targ.State = &store.ManifestState{LastSuccessfulDeployTime: time.Now()}
	state.UpsertManifestTarget(targ)

	nudge := NeedsNudge(*state)
	assert.False(t, nudge,
		"manifest is k8s_yaml, expected needsNudge = false")
}

func TestNeedsNudgeRedManifest(t *testing.T) {
	state := store.NewState()

	m := model.Manifest{Name: "server"}
	targ := store.NewManifestTarget(m)
	state.UpsertManifestTarget(targ)

	nudge := NeedsNudge(*state)
	assert.False(t, nudge,
		"manifest has never had successful build, expected needsNudge = false")
}

func TestNeedsNudgeGreenManifest(t *testing.T) {
	state := store.NewState()

	m := model.Manifest{Name: "server"}
	targ := store.NewManifestTarget(m)
	targ.State = &store.ManifestState{LastSuccessfulDeployTime: time.Now()}
	state.UpsertManifestTarget(targ)

	nudge := NeedsNudge(*state)
	assert.True(t, nudge,
		"manifest HAS had had successful build, expected needsNudge = true")
}

func TestNeedsNudgeAlreadyOpted(t *testing.T) {
	state := store.NewState()

	state.AnalyticsOpt = analytics.OptIn
	m := model.Manifest{Name: "server"}
	targ := store.NewManifestTarget(m)
	targ.State = &store.ManifestState{LastSuccessfulDeployTime: time.Now()}
	state.UpsertManifestTarget(targ)

	nudge := NeedsNudge(*state)
	assert.False(t, nudge,
		"user already opted in, expected needsNudge = false")
}

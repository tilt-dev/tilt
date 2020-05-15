package webview

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tilt-dev/wmclient/pkg/analytics"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestNeedsNudgeK8sYaml(t *testing.T) {
	state := newState(nil)

	m, err := k8s.NewK8sOnlyManifestFromYAML(testyaml.SanchoYAML)
	assert.NoError(t, err)
	targ := store.NewManifestTarget(m)
	targ.State = &store.ManifestState{LastSuccessfulDeployTime: time.Now()}
	state.UpsertManifestTarget(targ)

	nudge := NeedsNudge(*state)
	assert.False(t, nudge,
		"manifest is k8s_yaml, expected needsNudge = false")
}

func TestNeedsNudgeRedManifest(t *testing.T) {
	state := newState(nil)

	m := model.Manifest{Name: "server"}
	targ := store.NewManifestTarget(m)
	state.UpsertManifestTarget(targ)

	nudge := NeedsNudge(*state)
	assert.False(t, nudge,
		"manifest has never had successful build, expected needsNudge = false")
}

func TestNeedsNudgeGreenManifest(t *testing.T) {
	state := newState(nil)

	m := model.Manifest{Name: "server"}
	targ := store.NewManifestTarget(m)
	targ.State = &store.ManifestState{LastSuccessfulDeployTime: time.Now()}
	state.UpsertManifestTarget(targ)

	nudge := NeedsNudge(*state)
	assert.True(t, nudge,
		"manifest HAS had had successful build, expected needsNudge = true")
}

func TestNeedsNudgeAlreadyOpted(t *testing.T) {
	state := newState(nil)

	state.AnalyticsUserOpt = analytics.OptIn
	m := model.Manifest{Name: "server"}
	targ := store.NewManifestTarget(m)
	targ.State = &store.ManifestState{LastSuccessfulDeployTime: time.Now()}
	state.UpsertManifestTarget(targ)

	nudge := NeedsNudge(*state)
	assert.False(t, nudge,
		"user already opted in, expected needsNudge = false")
}

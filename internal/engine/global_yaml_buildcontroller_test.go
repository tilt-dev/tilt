package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/output"
)

func TestGlobalYAMLOnChange(t *testing.T) {
	st := newTestingStoreWithGlobalYAML(testyaml.DoggosServiceYaml)
	bc := newGlobalYamlBuildControllerForTest(testyaml.SecretYaml)

	bc.OnChange(output.CtxForTest(), st)

	assert.Equal(t, testyaml.DoggosServiceYaml, bc.lastGlobalYAMLManifest.K8sYAML())

	expectedActions := []store.Action{
		GlobalYAMLApplyStartedAction{},
		GlobalYAMLApplyCompleteAction{},
	}
	assert.Equal(t, expectedActions, st.Actions)
}

func TestNoChangeToGlobalYAML(t *testing.T) {
	st := newTestingStoreWithGlobalYAML(testyaml.SecretYaml)
	bc := newGlobalYamlBuildControllerForTest(testyaml.SecretYaml)

	bc.OnChange(output.CtxForTest(), st)

	assert.Equal(t, testyaml.SecretYaml, bc.lastGlobalYAMLManifest.K8sYAML())
	assert.Empty(t, st.Actions, "expect unchanged global yaml to dispatch no actions")
}

func TestGlobalYamlParseError(t *testing.T) {
	st := newTestingStoreWithGlobalYAML("some invalid yaml")
	bc := newGlobalYamlBuildControllerForTest(testyaml.SecretYaml)

	bc.OnChange(output.CtxForTest(), st)

	// even in an error case, we should update lastGlobalYAMLManifest
	// (so we don't try to re-apply the same bad yaml multiple times)
	assert.Equal(t, "some invalid yaml", bc.lastGlobalYAMLManifest.K8sYAML())

	if len(st.Actions) != 1 {
		t.Errorf("expect 1 action dispatched, got %d: %v", len(st.Actions), st.Actions)
	}
	gYAMLErr, ok := st.Actions[0].(GlobalYAMLApplyError)
	if !ok {
		t.Errorf("expected a `GlobalYAMLApplyError` error, got: %v", st.Actions[0])
	}
	assert.Contains(t, gYAMLErr.Error.Error(), "Error parsing global_yaml")
}

func newTestingStoreWithGlobalYAML(yaml string) *store.TestingStore {
	st := store.NewTestingStore()
	state := store.EngineState{
		GlobalYAML: model.NewYAMLManifest(model.GlobalYAMLManifestName, yaml, nil),
	}
	st.SetState(state)
	return st
}

func newGlobalYamlBuildControllerForTest(yaml string) *GlobalYAMLBuildController {
	return &GlobalYAMLBuildController{
		lastGlobalYAMLManifest: model.NewYAMLManifest(model.GlobalYAMLManifestName, yaml, nil),
		k8sClient:              k8s.NewFakeK8sClient(),
	}
}

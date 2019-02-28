package engine

import (
	"testing"

	"github.com/windmilleng/tilt/internal/yaml"

	"github.com/pkg/errors"
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

	assert.Equal(t, testyaml.DoggosServiceYaml, bc.lastGlobalYAMLManifest.K8sTarget().YAML)

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

	assert.Equal(t, testyaml.SecretYaml, bc.lastGlobalYAMLManifest.K8sTarget().YAML)
	assert.Empty(t, st.Actions, "expect unchanged global yaml to dispatch no actions")
}

func TestGlobalYamlParseError(t *testing.T) {
	st := newTestingStoreWithGlobalYAML("some invalid yaml")
	bc := newGlobalYamlBuildControllerForTest(testyaml.SecretYaml)

	bc.OnChange(output.CtxForTest(), st)

	// even in an error case, we should update lastGlobalYAMLManifest
	// (so we don't try to re-apply the same bad yaml multiple times)
	assert.Equal(t, "some invalid yaml", bc.lastGlobalYAMLManifest.K8sTarget().YAML)

	if len(st.Actions) != 2 {
		t.Errorf("expect 2 action dispatched, got %d: %#v", len(st.Actions), st.Actions)
		return
	}
	_, ok := st.Actions[0].(GlobalYAMLApplyStartedAction)
	if !ok {
		t.Errorf("first action should be a GlobalYAMLApplyStartedAction, got: %#v", st.Actions[0])
	}
	gYAMLErr, ok := st.Actions[1].(GlobalYAMLApplyCompleteAction)
	if !ok {
		t.Errorf("expected a `GlobalYAMLApplyCompleteAction` action, got: %#v", st.Actions[1])
		return
	}
	assert.Contains(t, gYAMLErr.Error.Error(), "Error parsing k8s_yaml")
}

func TestGlobalYamlFailUpsert(t *testing.T) {
	yaml := "apiVersion: v1\nKind: Service"
	st := newTestingStoreWithGlobalYAML(yaml)
	bc := newGlobalYamlBuildControllerForTest(testyaml.SecretYaml)

	bc.k8sClient.UpsertError = errors.New("upsert error!")
	bc.OnChange(output.CtxForTest(), st)

	assert.Equal(t, yaml, bc.lastGlobalYAMLManifest.K8sTarget().YAML)
	if len(st.Actions) != 2 {
		t.Errorf("expect 2 action dispatched, got %d: %#v", len(st.Actions), st.Actions)
		return
	}
	_, ok := st.Actions[0].(GlobalYAMLApplyStartedAction)
	if !ok {
		t.Errorf("first action should be a GlobalYAMLApplyStartedAction, got: %#v", st.Actions[0])
	}
	gYAMLErr, ok := st.Actions[1].(GlobalYAMLApplyCompleteAction)
	if !ok {
		t.Errorf("expected a `GlobalYAMLApplyCompleteAction` action, got: %#v", st.Actions[1])
		return
	}

	if assert.Error(t, gYAMLErr.Error) {
		assert.Contains(t, gYAMLErr.Error.Error(), bc.k8sClient.UpsertError.Error())
	}
}

func TestGlobalYamlNamespacesFirst(t *testing.T) {
	y := yaml.ConcatYAML(testyaml.DoggosDeploymentYaml, testyaml.MyNamespaceYAML)
	st := newTestingStoreWithGlobalYAML(y)
	bc := newGlobalYamlBuildControllerForTest(testyaml.SecretYaml)

	bc.OnChange(output.CtxForTest(), st)

	entities, err := k8s.ParseYAMLFromString(bc.k8sClient.Yaml)
	if err != nil {
		t.Fatal(err)
	}

	var observedNames []string
	for _, e := range entities {
		observedNames = append(observedNames, e.Name())
	}
	expectedNames := []string{"mynamespace", "doggos"}
	assert.Equal(t, expectedNames, observedNames)
}

func newTestingStoreWithGlobalYAML(yaml string) *store.TestingStore {
	st := store.NewTestingStore()
	state := store.EngineState{
		GlobalYAML: k8s.NewK8sOnlyManifestForTesting(model.GlobalYAMLManifestName, yaml),
	}
	st.SetState(state)
	return st
}

type TestGlobalYAMLBuildController struct {
	GlobalYAMLBuildController
	k8sClient *k8s.FakeK8sClient
}

func newGlobalYamlBuildControllerForTest(yaml string) *TestGlobalYAMLBuildController {
	kc := k8s.NewFakeK8sClient()
	return &TestGlobalYAMLBuildController{
		GlobalYAMLBuildController: GlobalYAMLBuildController{
			lastGlobalYAMLManifest: k8s.NewK8sOnlyManifestForTesting(model.GlobalYAMLManifestName, yaml),
			k8sClient:              kc,
		},
		k8sClient: kc,
	}
}

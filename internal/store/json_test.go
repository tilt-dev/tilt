package store

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestToJSON(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()

	m := manifestbuilder.New(f, "fe").
		WithK8sYAML(testyaml.SanchoYAML).
		Build()
	state := newState([]model.Manifest{m})

	mState, _ := state.ManifestState("fe")
	mState.MutableBuildStatus(m.K8sTarget().ID()).LastResult = NewK8sDeployResult(
		m.K8sTarget().ID(), &k8sconv.KubernetesApplyFilter{})

	buf := bytes.NewBuffer(nil)
	encoder := CreateEngineStateEncoder(buf)
	err := encoder.Encode(state)
	if err != nil {
		t.Fatal(err)
	}

	assert.Contains(t, buf.String(), "yaml")
	assert.Contains(t, buf.String(), "kind: Deployment")

	// Make sure the data can decode successfully.
	decoder := json.NewDecoder(bytes.NewBufferString(buf.String()))
	var v interface{}
	err = decoder.Decode(&v)
	if err != nil {
		t.Fatalf("Error decoding JSON: %v\nSource:\n%s\n", err, buf.String())
	}
}

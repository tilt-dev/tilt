package k8s

import (
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/apps/v1"

	"github.com/windmilleng/tilt/internal/k8s/testyaml"
)

func TestInjectPodTemplateHash(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.SanchoYAML)
	if err != nil {
		t.Fatal(err)
	}

	if len(entities) != 1 {
		t.Fatalf("Unexpected entities: %+v", entities)
	}

	orig := entities[0]
	preInjectOrigYAML, err := SerializeSpecYAML([]K8sEntity{orig})
	require.NoError(t, err)

	injected, err := InjectPodTemplateSpecHashes(orig)
	require.NoError(t, err)

	// make sure we haven't mutated the original object
	postInjectOrigYAML, err := SerializeSpecYAML([]K8sEntity{orig})
	require.NoError(t, err)
	require.Equal(t, preInjectOrigYAML, postInjectOrigYAML)

	// make sure the label is set and it's some kind of hash
	dep := injected.Obj.(*v1.Deployment)
	require.Regexp(t, `^[0-9a-f]{10,}$`, dep.Spec.Template.Labels[TiltPodTemplateHashLabel])
}

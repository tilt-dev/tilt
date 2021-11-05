package k8s

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
)

func TestNewTargetSortsK8sEntities(t *testing.T) {
	targ, err := NewTargetForYAML("foo", testyaml.OutOfOrderYaml, nil)
	require.NoError(t, err)

	expectedKindOrder := []string{"PersistentVolume", "PersistentVolumeClaim", "ConfigMap", "Service", "StatefulSet", "Job", "Pod"}

	actual, err := ParseYAMLFromString(targ.YAML)
	require.NoError(t, err)

	assertKindOrder(t, expectedKindOrder, actual, "result of `NewTarget` should contain sorted YAML")
}

package k8s

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestNewTargetSortsK8sEntities(t *testing.T) {
	entities := MustParseYAMLFromString(t, testyaml.OutOfOrderYaml)
	targ, err := NewTarget("foo", entities, nil, nil, nil, nil,
		nil, model.PodReadinessWait, v1alpha1.KubernetesDiscoveryStrategyDefault, nil, nil, model.UpdateSettings{})
	require.NoError(t, err)

	expectedKindOrder := []string{"PersistentVolume", "PersistentVolumeClaim", "ConfigMap", "Service", "StatefulSet", "Job", "Pod"}

	actual, err := ParseYAMLFromString(targ.YAML)
	require.NoError(t, err)

	assertKindOrder(t, expectedKindOrder, actual, "result of `NewTarget` should contain sorted YAML")
}

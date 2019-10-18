package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
)

func TestNewTargetSortsK8sEntities(t *testing.T) {
	entities := MustParseYAMLFromString(t, testyaml.PostgresYAML)
	entities[1], entities[3] = entities[3], entities[1] // swap some stuff around for more significant sorting
	targ, err := NewTarget("foo", entities, nil, nil, nil, nil)
	require.NoError(t, err)

	expectedKindOrder := []string{"PersistentVolume", "PersistentVolumeClaim", "ConfigMap", "Service", "StatefulSet"}

	actual, err := ParseYAMLFromString(targ.YAML)
	require.NoError(t, err)

	assertKindOrder(t, expectedKindOrder, actual, "result of `NewTarget` should contain sorted YAML")

	expectedDisplayNames := []string{
		"postgres-pv-volume:persistentvolume",
		"postgres-pv-claim:persistentvolumeclaim",
		"postgres-config:configmap",
		"postgres:service",
		"postgres:statefulset",
	}
	assert.Equal(t, expectedDisplayNames, targ.DisplayNames)
}

package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
)

func TestStatefulsetPodManagementPolicy(t *testing.T) {
	ss, err := ParseYAMLFromString(testyaml.RedisStatefulSetYAML)
	assert.Nil(t, err)

	newEntity := InjectParallelPodManagementPolicy(ss[0])
	result, err := SerializeSpecYAML([]K8sEntity{newEntity})
	if err != nil {
		t.Fatal(err)
	}
	assert.Contains(t, result, "podManagementPolicy: Parallel")
}

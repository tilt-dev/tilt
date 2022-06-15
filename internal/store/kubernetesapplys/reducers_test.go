package kubernetesapplys

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/kubernetesdiscoverys"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

const resultYAML = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: postgres-config
  uid: abc-123
  labels:
    app: postgres
data:
  POSTGRES_DB: postgresdb
  POSTGRES_USER: postgresadmin
  POSTGRES_PASSWORD: admin123
`

func TestCacheFilter(t *testing.T) {
	ka := &v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{Name: "a"},
		Status: v1alpha1.KubernetesApplyStatus{
			ResultYAML:    resultYAML,
			LastApplyTime: apis.NowMicro(),
		},
	}
	kd := &v1alpha1.KubernetesDiscovery{ObjectMeta: metav1.ObjectMeta{Name: "a"}}

	state := store.NewState()
	HandleKubernetesApplyUpsertAction(state, KubernetesApplyUpsertAction{
		KubernetesApply: ka,
	})
	kubernetesdiscoverys.HandleKubernetesDiscoveryUpsertAction(state, kubernetesdiscoverys.KubernetesDiscoveryUpsertAction{
		KubernetesDiscovery: kd,
	})

	res := state.KubernetesResources[kd.Name]
	assert.Equal(t, kd, res.Discovery)

	ka2 := ka.DeepCopy()
	ka2.Status.LastApplyTime = apis.NewMicroTime(time.Now().Add(time.Second))
	HandleKubernetesApplyUpsertAction(state, KubernetesApplyUpsertAction{
		KubernetesApply: ka2,
	})
	assert.Same(t, state.KubernetesResources[kd.Name].ApplyFilter, res.ApplyFilter)
}

func TestOutOfOrderSegfault(t *testing.T) {
	ka := &v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{Name: "a"},
		Status: v1alpha1.KubernetesApplyStatus{
			ResultYAML:    resultYAML,
			LastApplyTime: apis.NowMicro(),
		},
	}
	kd := &v1alpha1.KubernetesDiscovery{ObjectMeta: metav1.ObjectMeta{Name: "a"}}

	state := store.NewState()
	kubernetesdiscoverys.HandleKubernetesDiscoveryUpsertAction(state, kubernetesdiscoverys.KubernetesDiscoveryUpsertAction{
		KubernetesDiscovery: kd,
	})
	res := state.KubernetesResources[kd.Name]
	assert.Equal(t, kd, res.Discovery)
	assert.Nil(t, res.ApplyFilter)

	HandleKubernetesApplyUpsertAction(state, KubernetesApplyUpsertAction{
		KubernetesApply: ka,
	})
	assert.NotNil(t, state.KubernetesResources[kd.Name].ApplyFilter)
}

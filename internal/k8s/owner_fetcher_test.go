package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestVisitOneParent(t *testing.T) {
	kCli := &FakeK8sClient{}
	ov := ProvideOwnerFetcher(kCli)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-a",
			UID:  "pod-a-uid",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "rs-a",
					UID:        "rs-a-uid",
				},
			},
		},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rs-a",
			UID:  "rs-a-uid",
		},
	}
	kCli.InjectEntityByName(NewK8sEntity(rs))

	tree, err := ov.OwnerTreeOf(K8sEntity{Obj: pod})
	assert.NoError(t, err)
	assert.Equal(t, `Pod:pod-a
  ReplicaSet:rs-a`, tree.String())
}

func TestVisitTwoParents(t *testing.T) {
	kCli := &FakeK8sClient{}
	ov := ProvideOwnerFetcher(kCli)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-a",
			UID:  "pod-a-uid",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "rs-a",
					UID:        "rs-a-uid",
				},
			},
		},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rs-a",
			UID:  "rs-a-uid",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "dep-a",
					UID:        "dep-a-uid",
				},
			},
		},
	}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dep-a",
			UID:  "dep-a-uid",
		},
	}
	kCli.InjectEntityByName(NewK8sEntity(rs), NewK8sEntity(dep))

	tree, err := ov.OwnerTreeOf(K8sEntity{Obj: pod})
	assert.NoError(t, err)
	assert.Equal(t, `Pod:pod-a
  ReplicaSet:rs-a
    Deployment:dep-a`, tree.String())
}

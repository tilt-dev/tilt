package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestVisitOneParent(t *testing.T) {
	kCli := NewFakeK8sClient(t)
	ov := NewOwnerFetcher(context.Background(), kCli)

	pod, rs := fakeOneParentChain()
	kCli.Inject(NewK8sEntity(rs))

	tree, err := ov.OwnerTreeOf(context.Background(), NewK8sEntity(pod))
	assert.NoError(t, err)
	assert.Equal(t, `Pod:pod-a
  ReplicaSet:rs-a`, tree.String())
}

func TestVisitTwoParentsEnsureListCaching(t *testing.T) {
	kCli := NewFakeK8sClient(t)
	ov := NewOwnerFetcher(context.Background(), kCli)

	pod, rs, dep := fakeTwoParentChain()
	kCli.Inject(NewK8sEntity(rs), NewK8sEntity(dep))

	tree, err := ov.OwnerTreeOf(context.Background(), NewK8sEntity(pod))
	assert.NoError(t, err)
	assert.Equal(t, `Pod:pod-a
  ReplicaSet:rs-a
    Deployment:dep-a`, tree.String())
	assert.Equal(t, 2, kCli.listCallCount)
	assert.Equal(t, 0, kCli.getByReferenceCallCount)
}

func TestVisitTwoParentsNoList(t *testing.T) {
	kCli := NewFakeK8sClient(t)
	kCli.listReturnsEmpty = true
	ov := NewOwnerFetcher(context.Background(), kCli)

	pod, rs, dep := fakeTwoParentChain()
	kCli.Inject(NewK8sEntity(rs), NewK8sEntity(dep))

	tree, err := ov.OwnerTreeOf(context.Background(), NewK8sEntity(pod))
	assert.NoError(t, err)
	assert.Equal(t, `Pod:pod-a
  ReplicaSet:rs-a
    Deployment:dep-a`, tree.String())
	assert.Equal(t, 2, kCli.listCallCount)
	assert.Equal(t, 2, kCli.getByReferenceCallCount)
}

func TestOwnerFetcherParallelism(t *testing.T) {
	kCli := NewFakeK8sClient(t)
	kCli.listReturnsEmpty = true
	ov := NewOwnerFetcher(context.Background(), kCli)

	pod, rs := fakeOneParentChain()
	kCli.Inject(NewK8sEntity(rs))

	count := 30
	g, ctx := errgroup.WithContext(context.Background())
	for i := 0; i < count; i++ {
		g.Go(func() error {
			_, err := ov.OwnerTreeOf(ctx, NewK8sEntity(pod))
			return err
		})
	}

	err := g.Wait()
	assert.NoError(t, err)
	assert.Equal(t, 1, kCli.getByReferenceCallCount)
}

func TestCircular(t *testing.T) {
	kCli := NewFakeK8sClient(t)
	kCli.listReturnsEmpty = true
	ov := NewOwnerFetcher(context.Background(), kCli)

	pod1, pod2, pod3 := fakeCircularReference()
	kCli.Inject(NewK8sEntity(pod2), NewK8sEntity(pod3))

	tree, err := ov.OwnerTreeOf(context.Background(), NewK8sEntity(pod1))
	assert.NoError(t, err)
	assert.Equal(t, `Pod:pod-a
  Pod:pod-b
    Pod:pod-c`, tree.String())
}

func fakeOneParentChain() (*v1.Pod, *appsv1.ReplicaSet) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-a",
			UID:       "pod-a-uid",
			Namespace: "default",
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
			Name:      "rs-a",
			UID:       "rs-a-uid",
			Namespace: "default",
		},
	}
	return pod, rs
}

func fakeTwoParentChain() (*v1.Pod, *appsv1.ReplicaSet, *appsv1.Deployment) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-a",
			UID:       "pod-a-uid",
			Namespace: "default",
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
			Name:      "rs-a",
			UID:       "rs-a-uid",
			Namespace: "default",
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
			Name:      "dep-a",
			UID:       "dep-a-uid",
			Namespace: "default",
		},
	}
	return pod, rs, dep
}

func fakeCircularReference() (*v1.Pod, *v1.Pod, *v1.Pod) {
	pod1 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-a",
			UID:       "pod-a-uid",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "pod-b",
					UID:        "pod-b-uid",
				},
			},
		},
	}
	pod2 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-b",
			UID:       "pod-b-uid",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "pod-c",
					UID:        "pod-c-uid",
				},
			},
		},
	}
	pod3 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-c",
			UID:       "pod-c-uid",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "pod-a",
					UID:        "pod-a-uid",
				},
			},
		},
	}

	return pod1, pod2, pod3
}

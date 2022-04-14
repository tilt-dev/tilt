package cluster

import (
	"errors"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/timecmp"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestClientManager_GetK8sClient_Success(t *testing.T) {
	f := newCmFixture(t)
	c := f.setupSuccessCluster()

	obj := newFakeObj()
	clusterRef := clusterRef{clusterKey: apis.Key(c), objKey: apis.Key(obj)}
	// ensure we can retrieve the client multiple times
	for i := 0; i < 5; i++ {
		cli, err := f.cm.GetK8sClient(obj, c)
		require.NoError(t, err)
		require.NotNil(t, cli)

		// manager should be tracking revision and it shouldn't change
		timecmp.AssertTimeEqual(t, c.Status.ConnectedAt, f.cm.revisions[clusterRef])
	}
}

func TestClientManager_GetK8sClient_NotFound(t *testing.T) {
	f := newCmFixture(t)
	// create a stub cluster obj that's not known by the ClientProvider
	c := newCluster()

	obj := newFakeObj()
	cli, err := f.cm.GetK8sClient(obj, c)
	require.EqualError(t, err, "cluster client does not exist")
	require.Nil(t, cli)

	// no revision should have been stored
	require.Empty(t, f.cm.revisions)
}

func TestClientManager_GetK8sClient_Stale(t *testing.T) {
	f := newCmFixture(t)
	c := f.setupSuccessCluster()

	obj := newFakeObj()
	old := apis.NewMicroTime(c.Status.ConnectedAt.Add(-time.Second))
	c.Status.ConnectedAt = &old
	cli, err := f.cm.GetK8sClient(obj, c)
	require.EqualError(t, err, "cluster revision is stale")
	require.Nil(t, cli)

	// no revision should have been stored
	require.Empty(t, f.cm.revisions)
}

func TestClientManager_GetK8sClient_Error(t *testing.T) {
	f := newCmFixture(t)
	c := f.setupErrorCluster("oh no")

	obj := newFakeObj()
	cli, err := f.cm.GetK8sClient(obj, c)
	require.EqualError(t, err, "oh no")
	require.Nil(t, cli)

	// no revision should have been stored
	require.Empty(t, f.cm.revisions)
}

func TestClientManager_GetK8sClient_Refresh(t *testing.T) {
	f := newCmFixture(t)
	origCluster := f.setupSuccessCluster()

	objA := newFakeObj()
	objB := newFakeObj()

	// fetch once for each obj so that it's tracked
	for _, obj := range []apis.KeyableObject{objA, objB} {
		cli, err := f.cm.GetK8sClient(obj, origCluster)
		require.NoError(t, err)
		require.NotNil(t, cli)
	}

	// update the client
	updatedCluster := origCluster.DeepCopy()
	newRevision := f.cp.SetK8sClient(apis.Key(updatedCluster), k8s.NewFakeK8sClient(t))
	updatedCluster.Status.ConnectedAt = newRevision.DeepCopy()

	for _, obj := range []apis.KeyableObject{objA, objB} {
		clusterRef := clusterRef{clusterKey: apis.Key(origCluster), objKey: apis.Key(obj)}

		cli, err := f.cm.GetK8sClient(obj, origCluster)
		require.EqualError(t, err, "cluster revision is stale")
		require.Nil(t, cli)

		// if we pass in the stale cluster object, no refresh should happen
		require.False(t, f.cm.Refresh(obj, origCluster), "Refresh should return false with original cluster")

		// revision is still the old one
		timecmp.AssertTimeEqual(t, origCluster.Status.ConnectedAt, f.cm.revisions[clusterRef])

		// once we pass in the updated cluster object, a refresh SHOULD happen
		// note that this happens for BOTH objA and objB because they might have
		// independently managed state for the cluster!
		require.True(t, f.cm.Refresh(obj, updatedCluster), "Refresh should have been triggered")

		// since the refresh occurred, the revision should have been wiped
		require.Zero(t, f.cm.revisions[clusterRef], "Revision should have been forgotten")

		// we can get the new client now!
		cli, err = f.cm.GetK8sClient(obj, updatedCluster)
		require.NoError(t, err)
		require.NotNil(t, cli)

		// revision is now the new one
		timecmp.AssertTimeEqual(t, updatedCluster.Status.ConnectedAt, f.cm.revisions[clusterRef])
	}
}

type cmFixture struct {
	t  testing.TB
	cp *FakeClientProvider
	cm *ClientManager
}

func newCmFixture(t testing.TB) *cmFixture {
	cp := NewFakeClientProvider(t, fake.NewFakeTiltClient())
	return &cmFixture{
		t:  t,
		cp: cp,
		cm: NewClientManager(cp),
	}
}

type fakeObj struct {
	name      string
	namespace string
}

var _ apis.KeyableObject = fakeObj{}

func (f fakeObj) GetName() string {
	return f.name
}

func (f fakeObj) GetNamespace() string {
	return f.namespace
}

func newFakeObj() fakeObj {
	return fakeObj{
		namespace: strconv.Itoa(rand.Int()),
		name:      strconv.Itoa(rand.Int()),
	}
}

func (f *cmFixture) setupSuccessCluster() *v1alpha1.Cluster {
	c := newCluster()
	c.Status.Arch = "amd64"

	// get the timestamp from the fake client provider and set it
	ts := f.cp.SetK8sClient(apis.Key(c), k8s.NewFakeK8sClient(f.t))
	c.Status.ConnectedAt = ts.DeepCopy()

	return c
}

func (f *cmFixture) setupErrorCluster(error string) *v1alpha1.Cluster {
	c := newCluster()
	c.Status.Error = error

	f.cp.SetClusterError(apis.Key(c), errors.New(error))
	return c
}

func newCluster() *v1alpha1.Cluster {
	c := &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: strconv.Itoa(rand.Int()),
			Name:      strconv.Itoa(rand.Int()),
		},
	}
	return c
}

package localregistry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	ktesting "k8s.io/client-go/testing"
)

func TestDiscover(t *testing.T) {
	cs := &fake.Clientset{}
	tracker := ktesting.NewObjectTracker(scheme.Scheme, scheme.Codecs.UniversalDecoder())
	cs.AddReactor("*", "*", ktesting.ObjectReaction(tracker))

	obj, _, err :=
		scheme.Codecs.UniversalDeserializer().Decode([]byte(SampleConfigMap), nil, nil)
	require.NoError(t, err)
	err = tracker.Add(obj)
	require.NoError(t, err)

	core := cs.CoreV1()
	hosting, err := Discover(context.Background(), core)
	require.NoError(t, err)

	assert.Equal(t, hosting, LocalRegistryHostingV1{
		Host:                     "localhost:5000",
		Help:                     "https://kind.sigs.k8s.io/docs/user/local-registry/",
		HostFromContainerRuntime: "registry:5000",
		HostFromClusterNetwork:   "kind-registry:5000",
	})
}

func TestDiscoverNotFound(t *testing.T) {
	cs := &fake.Clientset{}
	tracker := ktesting.NewObjectTracker(scheme.Scheme, scheme.Codecs.UniversalDecoder())
	cs.AddReactor("*", "*", ktesting.ObjectReaction(tracker))

	core := cs.CoreV1()
	hosting, err := Discover(context.Background(), core)

	require.NoError(t, err)
	assert.Equal(t, LocalRegistryHostingV1{}, hosting)
}

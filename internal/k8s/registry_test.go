package k8s

import (
	"bytes"
	"context"
	"io"
	"os"
	goruntime "runtime"
	"testing"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/pkg/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	ktesting "k8s.io/client-go/testing"
)

func TestRegistryFoundMicrok8s(t *testing.T) {
	// microk8s is linux-only
	if goruntime.GOOS != "linux" {
		t.SkipNow()
	}

	cs := &fake.Clientset{}
	tracker := ktesting.NewObjectTracker(scheme.Scheme, scheme.Codecs.UniversalDecoder())
	cs.AddReactor("*", "*", ktesting.ObjectReaction(tracker))
	_ = tracker.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      microk8sRegistryName,
			Namespace: microk8sRegistryNamespace,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				v1.ServicePort{NodePort: 32000},
			},
		},
	})

	core := cs.CoreV1()
	registryAsync := newRegistryAsync(EnvMicroK8s, core, NewNaiveRuntimeSource(container.RuntimeContainerd))

	registry := registryAsync.Registry(newLoggerCtx(os.Stdout))
	assert.Equal(t, "localhost:32000", registry.Host)
}

func TestRegistryFoundInTiltAnnotationsWithClusterHost(t *testing.T) {
	cs := &fake.Clientset{}
	tracker := ktesting.NewObjectTracker(scheme.Scheme, scheme.Codecs.UniversalDecoder())
	cs.AddReactor("*", "*", ktesting.ObjectReaction(tracker))
	_ = tracker.Add(&v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
			Annotations: map[string]string{
				tiltAnnotationRegistry:            "localhost:5000",
				tiltAnnotationRegistryFromCluster: "registry:5000",
			},
		},
	})

	core := cs.CoreV1()
	registryAsync := newRegistryAsync(EnvKIND6, core, NewNaiveRuntimeSource(container.RuntimeContainerd))

	registry := registryAsync.Registry(newLoggerCtx(os.Stdout))
	assert.Equal(t, "localhost:5000", registry.Host)
	assert.Equal(t, "registry:5000", registry.HostFromCluster())
}

func TestRegistryFoundInKindAnnotations(t *testing.T) {
	cs := &fake.Clientset{}
	tracker := ktesting.NewObjectTracker(scheme.Scheme, scheme.Codecs.UniversalDecoder())
	cs.AddReactor("*", "*", ktesting.ObjectReaction(tracker))
	_ = tracker.Add(&v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
			Annotations: map[string]string{
				kindAnnotationRegistry: "localhost:5000",
			},
		},
	})

	core := cs.CoreV1()
	registryAsync := newRegistryAsync(EnvKIND6, core, NewNaiveRuntimeSource(container.RuntimeContainerd))

	registry := registryAsync.Registry(newLoggerCtx(os.Stdout))
	assert.Equal(t, "localhost:5000", registry.Host)
}

func TestLocalRegistryDiscoveryHelp(t *testing.T) {
	cs := &fake.Clientset{}
	tracker := ktesting.NewObjectTracker(scheme.Scheme, scheme.Codecs.UniversalDecoder())
	cs.AddReactor("*", "*", ktesting.ObjectReaction(tracker))
	err := addConfigMap(tracker, `
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    help: "https://fake-domain.tilt.dev/local-registry-help"
`)
	require.NoError(t, err)

	core := cs.CoreV1()
	registryAsync := newRegistryAsync(EnvKIND6, core, NewNaiveRuntimeSource(container.RuntimeContainerd))

	out := bytes.NewBuffer(nil)
	registry := registryAsync.Registry(newLoggerCtx(out))
	assert.True(t, registry.Empty())
	assert.Contains(t, out.String(), "https://fake-domain.tilt.dev/local-registry-help")
}

func TestLocalRegistryDiscoveryHost(t *testing.T) {
	cs := &fake.Clientset{}
	tracker := ktesting.NewObjectTracker(scheme.Scheme, scheme.Codecs.UniversalDecoder())
	cs.AddReactor("*", "*", ktesting.ObjectReaction(tracker))
	err := addConfigMap(tracker, `
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:5000"
    hostFromContainerRuntime: "registry:5000"
    help: "https://fake-domain.tilt.dev/local-registry-help"
`)
	require.NoError(t, err)

	core := cs.CoreV1()
	registryAsync := newRegistryAsync(EnvKIND6, core, NewNaiveRuntimeSource(container.RuntimeContainerd))

	registry := registryAsync.Registry(newLoggerCtx(os.Stdout))
	assert.Equal(t, "localhost:5000", registry.Host)
	assert.Equal(t, "registry:5000", registry.HostFromCluster())
}

func TestKINDWarning(t *testing.T) {
	cs := &fake.Clientset{}
	core := cs.CoreV1()
	registryAsync := newRegistryAsync(EnvKIND6, core, NewNaiveRuntimeSource(container.RuntimeContainerd))

	out := bytes.NewBuffer(nil)
	registry := registryAsync.Registry(newLoggerCtx(out))
	assert.True(t, registry.Empty())
	assert.Contains(t, out.String(), "https://github.com/tilt-dev/kind-local")
}

func TestK3DWarning(t *testing.T) {
	cs := &fake.Clientset{}
	core := cs.CoreV1()
	registryAsync := newRegistryAsync(EnvK3D, core, NewNaiveRuntimeSource(container.RuntimeContainerd))

	out := bytes.NewBuffer(nil)
	registry := registryAsync.Registry(newLoggerCtx(out))
	assert.True(t, registry.Empty())
	assert.Contains(t, out.String(), "https://github.com/tilt-dev/k3d-local-registry")
}

func TestRegistryFoundInLabelsWithLocalOnly(t *testing.T) {
	cs := &fake.Clientset{}
	tracker := ktesting.NewObjectTracker(scheme.Scheme, scheme.Codecs.UniversalDecoder())
	cs.AddReactor("*", "*", ktesting.ObjectReaction(tracker))
	_ = tracker.Add(&v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
			Annotations: map[string]string{
				tiltAnnotationRegistry: "localhost:5000",
			},
		},
	})

	core := cs.CoreV1()
	registryAsync := newRegistryAsync(EnvKIND6, core, NewNaiveRuntimeSource(container.RuntimeContainerd))

	registry := registryAsync.Registry(newLoggerCtx(os.Stdout))
	assert.Equal(t, "localhost:5000", registry.Host)
	assert.Equal(t, "localhost:5000", registry.HostFromCluster())
}

func TestRegistryNotFound(t *testing.T) {
	// microk8s is linux-only
	if goruntime.GOOS != "linux" {
		t.SkipNow()
	}

	cs := &fake.Clientset{}
	tracker := ktesting.NewObjectTracker(scheme.Scheme, scheme.Codecs.UniversalDecoder())
	cs.AddReactor("*", "*", ktesting.ObjectReaction(tracker))

	core := cs.CoreV1()
	registryAsync := newRegistryAsync(EnvMicroK8s, core, NewNaiveRuntimeSource(container.RuntimeContainerd))

	out := bytes.NewBuffer(nil)
	registry := registryAsync.Registry(newLoggerCtx(out))
	assert.Equal(t, "", registry.Host)
	assert.Contains(t, out.String(), "microk8s.enable registry")
}

func newLoggerCtx(w io.Writer) context.Context {
	l := logger.NewLogger(logger.InfoLvl, w)
	ctx := logger.WithLogger(context.Background(), l)
	return ctx
}

func addConfigMap(tracker ktesting.ObjectTracker, configMap string) error {
	obj, _, err :=
		scheme.Codecs.UniversalDeserializer().Decode([]byte(configMap), nil, nil)
	if err != nil {
		return err
	}
	return tracker.Add(obj)
}

package k8s

import (
	"bytes"
	"context"
	goruntime "runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/pkg/logger"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestRegistryFound(t *testing.T) {
	// microk8s is linux-only
	if goruntime.GOOS != "linux" {
		t.SkipNow()
	}

	cs := &fake.Clientset{}
	cs.AddReactor("*", "*", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &v1.Service{
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					v1.ServicePort{NodePort: 32000},
				},
			},
		}, nil
	})

	core := cs.CoreV1()
	registryAsync := newRegistryAsync(EnvMicroK8s, core, NewNaiveRuntimeSource(container.RuntimeContainerd))

	out := &bytes.Buffer{}
	l := logger.NewLogger(logger.InfoLvl, out)
	ctx := logger.WithLogger(context.Background(), l)
	registry, err := registryAsync.Registry(ctx)
	require.NoError(t, err)
	assert.Equal(t, "localhost:32000", registry.Host)
}

func TestRegistryNotFound(t *testing.T) {
	// microk8s is linux-only
	if goruntime.GOOS != "linux" {
		t.SkipNow()
	}

	cs := &fake.Clientset{}
	cs.AddReactor("*", "*", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.NewNotFound(schema.GroupResource{Group: "core/v1", Resource: "service"}, "registry")
	})

	core := cs.CoreV1()
	registryAsync := newRegistryAsync(EnvMicroK8s, core, NewNaiveRuntimeSource(container.RuntimeContainerd))

	out := &bytes.Buffer{}
	l := logger.NewLogger(logger.InfoLvl, out)
	ctx := logger.WithLogger(context.Background(), l)
	registry, err := registryAsync.Registry(ctx)
	require.NoError(t, err)
	assert.Equal(t, "", registry.Host)
	assert.Contains(t, out.String(), "microk8s.enable registry")
}

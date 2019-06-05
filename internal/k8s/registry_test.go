package k8s

import (
	"bytes"
	"context"
	"testing"

	"github.com/windmilleng/tilt/internal/logger"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestRegistryFound(t *testing.T) {
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
	registryAsync := newRegistryAsync(EnvMicroK8s, core)

	out := &bytes.Buffer{}
	l := logger.NewLogger(logger.InfoLvl, out)
	ctx := logger.WithLogger(context.Background(), l)
	registry := registryAsync.Registry(ctx)
	assert.Equal(t, "localhost:32000", string(registry))
}

func TestRegistryNotFound(t *testing.T) {
	cs := &fake.Clientset{}
	cs.AddReactor("*", "*", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.NewNotFound(schema.GroupResource{Group: "core/v1", Resource: "service"}, "registry")
	})

	core := cs.CoreV1()
	registryAsync := newRegistryAsync(EnvMicroK8s, core)

	out := &bytes.Buffer{}
	l := logger.NewLogger(logger.InfoLvl, out)
	ctx := logger.WithLogger(context.Background(), l)
	registry := registryAsync.Registry(ctx)
	assert.Equal(t, "", string(registry))
	assert.Contains(t, out.String(), "microk8s.enable registry")
}

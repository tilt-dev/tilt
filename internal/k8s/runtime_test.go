package k8s

import (
	"bytes"
	"context"
	"testing"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/pkg/logger"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestRuntimeForbidden(t *testing.T) {
	cs := &fake.Clientset{}
	cs.AddReactor("*", "*", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, newForbiddenError()
	})

	core := cs.CoreV1()
	runtimeAsync := newRuntimeAsync(core)

	out := &bytes.Buffer{}
	l := logger.NewLogger(logger.InfoLvl, out)
	ctx := logger.WithLogger(context.Background(), l)
	runtime := runtimeAsync.Runtime(ctx)
	assert.Equal(t, container.RuntimeReadFailure, runtime)
	assert.Contains(t, out.String(), "Tilt could not read your node configuration")
}

package fake

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Helper methods for making updates.

func UpsertSpec(ctx context.Context, t testing.TB, ctrlClient ctrlclient.Client, obj ctrlclient.Object) {
	t.Helper()

	require.True(t, obj.GetName() != "", "object has empty name")
	err := ctrlClient.Create(ctx, obj)
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return
		}
		assert.Fail(t, "create failed", "%+v %+v", obj, err)
		return
	}

	copy := obj.DeepCopyObject().(ctrlclient.Object)
	err = ctrlClient.Get(ctx, types.NamespacedName{Name: obj.GetName()}, copy)
	if ctx.Err() != nil {
		return
	}
	assert.NoError(t, err)

	obj.SetResourceVersion(copy.GetResourceVersion())

	err = ctrlClient.Update(ctx, obj)
	if ctx.Err() != nil {
		return
	}
	assert.NoError(t, err)
}

func UpdateStatus(ctx context.Context, t testing.TB, ctrlClient ctrlclient.Client, obj ctrlclient.Object) {
	copy := obj.DeepCopyObject().(ctrlclient.Object)
	err := ctrlClient.Get(ctx, types.NamespacedName{Name: obj.GetName()}, copy)
	if ctx.Err() != nil {
		return
	}
	assert.NoError(t, err)

	obj.SetResourceVersion(copy.GetResourceVersion())

	err = ctrlClient.Status().Update(ctx, obj)
	if ctx.Err() != nil {
		return
	}
	assert.NoError(t, err)
}

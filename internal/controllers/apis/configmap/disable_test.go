package configmap

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

const configMapName = "fe-disable"
const key = "isDisabled"

func TestMaybeNewDisableStatusNoSource(t *testing.T) {
	f := newDisableFixture(t)
	newStatus, err := MaybeNewDisableStatus(f.ctx, f.fc, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, newStatus)
	require.Equal(t, false, newStatus.Disabled)
	require.Contains(t, newStatus.Reason, "does not specify a DisableSource")
}

func TestMaybeNewDisableStatusNoConfigMapDisableSource(t *testing.T) {
	f := newDisableFixture(t)
	newStatus, err := MaybeNewDisableStatus(f.ctx, f.fc, &v1alpha1.DisableSource{}, nil)
	require.NoError(t, err)
	require.NotNil(t, newStatus)
	require.Equal(t, false, newStatus.Disabled)
	require.Contains(t, newStatus.Reason, "specifies no ConfigMap")
}

func TestMaybeNewDisableStatusNoConfigMap(t *testing.T) {
	f := newDisableFixture(t)
	newStatus, err := MaybeNewDisableStatus(f.ctx, f.fc, disableSource(), nil)
	require.NoError(t, err)
	require.NotNil(t, newStatus)
	require.Equal(t, false, newStatus.Disabled)
	require.Contains(t, newStatus.Reason, "ConfigMap \"fe-disable\" does not exist")
}

func TestMaybeNewDisableStatusNoKey(t *testing.T) {
	f := newDisableFixture(t)
	f.createConfigMap(nil)
	newStatus, err := MaybeNewDisableStatus(f.ctx, f.fc, disableSource(), nil)
	require.NoError(t, err)
	require.NotNil(t, newStatus)
	require.Equal(t, false, newStatus.Disabled)
	require.Contains(t, newStatus.Reason, "has no key")
}

func TestMaybeNewDisableStatusTrue(t *testing.T) {
	f := newDisableFixture(t)
	f.createConfigMap(pointer.StringPtr("true"))
	newStatus, err := MaybeNewDisableStatus(f.ctx, f.fc, disableSource(), nil)
	require.NoError(t, err)
	require.NotNil(t, newStatus)
	require.Equal(t, true, newStatus.Disabled)
	require.Contains(t, newStatus.Reason, "is \"true\"")
}

func TestMaybeNewDisableStatusFalse(t *testing.T) {
	f := newDisableFixture(t)
	f.createConfigMap(pointer.StringPtr("false"))
	newStatus, err := MaybeNewDisableStatus(f.ctx, f.fc, disableSource(), nil)
	require.NoError(t, err)
	require.NotNil(t, newStatus)
	require.Equal(t, false, newStatus.Disabled)
	require.Contains(t, newStatus.Reason, "is not \"true\"")
}

func TestMaybeNewDisableStatusGobbledygookValue(t *testing.T) {
	f := newDisableFixture(t)
	f.createConfigMap(pointer.StringPtr("asdf"))
	newStatus, err := MaybeNewDisableStatus(f.ctx, f.fc, disableSource(), nil)
	require.NoError(t, err)
	require.NotNil(t, newStatus)
	require.Equal(t, false, newStatus.Disabled)
	require.Contains(t, newStatus.Reason, "is not \"true\"")
}

func TestMaybeNewDisableStatusNoChange(t *testing.T) {
	f := newDisableFixture(t)
	f.createConfigMap(pointer.StringPtr("false"))
	status, err := MaybeNewDisableStatus(f.ctx, f.fc, disableSource(), nil)
	require.NoError(t, err)
	newStatus, err := MaybeNewDisableStatus(f.ctx, f.fc, disableSource(), status)
	require.Same(t, status, newStatus)
}

func TestMaybeNewDisableStatusChange(t *testing.T) {
	f := newDisableFixture(t)
	f.createConfigMap(pointer.StringPtr("false"))
	status, err := MaybeNewDisableStatus(
		f.ctx,
		f.fc,
		disableSource(),
		nil,
	)
	require.NoError(t, err)
	f.updateConfigMap(pointer.StringPtr("true"))
	newStatus, err := MaybeNewDisableStatus(f.ctx, f.fc, disableSource(), status)
	require.NotSame(t, status, newStatus)
}

type disableFixture struct {
	t   *testing.T
	fc  ctrlclient.Client
	ctx context.Context
}

func (f *disableFixture) createConfigMap(isDisabled *string) {
	m := make(map[string]string)
	if isDisabled != nil {
		m[key] = *isDisabled
	}
	err := f.fc.Create(f.ctx, &v1alpha1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: configMapName,
		},
		Data: m,
	})

	require.NoError(f.t, err)
}

func (f *disableFixture) updateConfigMap(isDisabled *string) {
	m := make(map[string]string)
	if isDisabled != nil {
		m[key] = *isDisabled
	}
	var cm v1alpha1.ConfigMap
	err := f.fc.Get(f.ctx, types.NamespacedName{Name: configMapName}, &cm)
	require.NoError(f.t, err)

	cm.Data = m
	err = f.fc.Update(f.ctx, &cm)
	require.NoError(f.t, err)
}

func disableSource() *v1alpha1.DisableSource {
	return &v1alpha1.DisableSource{
		ConfigMap: &v1alpha1.ConfigMapDisableSource{
			Name: configMapName,
			Key:  key,
		},
	}
}

func newDisableFixture(t *testing.T) *disableFixture {
	fc := fake.NewFakeTiltClient()
	ctx := context.Background()
	return &disableFixture{
		t:   t,
		fc:  fc,
		ctx: ctx,
	}
}

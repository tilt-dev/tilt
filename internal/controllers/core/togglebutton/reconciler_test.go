package togglebutton

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

var tbName = types.NamespacedName{Name: "test-toggle"}

func TestReconciler_CreatesOffUIButton(t *testing.T) {
	f := newFixture(t)

	f.setupTest()

	uib := f.uiButton()
	require.Equal(t, "enable", uib.Spec.Text)

	f.requireSteadyState()
}

func TestReconciler_CreatesOnUIButton(t *testing.T) {
	f := newFixture(t)

	f.setupTest()

	cm := f.configMap()
	cm.Data["enabled"] = "true"
	require.NoError(t, f.Client.Update(f.ctx, &cm))

	f.MustReconcile(tbName)

	uib := f.uiButton()
	require.Equal(t, "disable", uib.Spec.Text)

	f.requireSteadyState()
}

func TestReconciler_DeletesUIButton(t *testing.T) {
	f := newFixture(t)

	f.setupTest()

	uib := f.uiButton()
	require.NotNil(t, uib)

	tb := f.toggleButton()
	found, _ := f.Delete(&tb)
	require.True(t, found)

	found = f.Get(f.KeyForObject(&uib), &uib)
	require.False(t, found)
}

func TestReconciler_HandlesClick(t *testing.T) {
	f := newFixture(t)

	f.setupTest()

	// simulate a click on the button
	uib := f.uiButton()
	require.NotNil(t, uib)
	uib.Status.LastClickedAt = metav1.NowMicro()
	uib.Status.Inputs = append(uib.Status.Inputs, v1alpha1.UIInputStatus{
		Name:   uib.Spec.Inputs[0].Name,
		Hidden: &v1alpha1.UIHiddenInputStatus{Value: uib.Spec.Inputs[0].Hidden.Value},
	})
	err := f.Client.Status().Update(f.ctx, &uib)
	require.NoError(t, err)

	f.MustReconcile(tbName)

	// now we should see:
	// 1. The ConfigMap gets updated
	cm := f.configMap()
	require.Equal(t, "true", cm.Data["enabled"])

	// 2. The UIButton reflects the TB's OnStateSpec
	uib = f.uiButton()
	require.Equal(t, "disable", uib.Spec.Text)

	f.requireSteadyState()
}

func TestReconciler_HandlesConfigMapUpdate(t *testing.T) {
	f := newFixture(t)

	f.setupTest()

	cm := f.configMap()
	cm.Data["enabled"] = "true"
	err := f.Client.Update(f.ctx, &cm)
	require.NoError(t, err)

	f.MustReconcile(tbName)

	// changing the configmap directly should cause the uibutton to update
	uib := f.uiButton()
	require.Equal(t, "disable", uib.Spec.Text)

	f.requireSteadyState()
}

func TestReconciler_uiButtonClickedNoInput(t *testing.T) {
	f := newFixture(t)

	f.setupTest()

	// simulate a click on the button, but without the expected input
	uib := f.uiButton()
	require.NotNil(t, uib)
	uib.Status.LastClickedAt = metav1.NowMicro()
	err := f.Client.Status().Update(f.ctx, &uib)
	require.NoError(t, err)

	f.MustReconcile(tbName)

	tb := f.toggleButton()
	require.Contains(t, tb.Status.Error, "does not have an input named \"action\"")

	f.requireSteadyState()
}

func TestReconciler_uiButtonClickedInputWrongType(t *testing.T) {
	f := newFixture(t)

	f.setupTest()

	// simulate a click on the button, but with the wrong input type
	uib := f.uiButton()
	require.NotNil(t, uib)
	uib.Status.LastClickedAt = metav1.NowMicro()
	uib.Status.Inputs = []v1alpha1.UIInputStatus{
		{
			Name: actionUIInputName,
			Text: &v1alpha1.UITextInputStatus{Value: turnOnInputValue},
		},
	}
	err := f.Client.Status().Update(f.ctx, &uib)
	require.NoError(t, err)

	f.MustReconcile(tbName)

	tb := f.toggleButton()
	require.Contains(t, tb.Status.Error, "input \"action\" was not of type 'Hidden'")

	f.requireSteadyState()
}

func TestReconciler_uiButtonClickedInputWrongValue(t *testing.T) {
	f := newFixture(t)

	f.setupTest()

	// simulate a click on the button, but with an unknown value
	uib := f.uiButton()
	require.NotNil(t, uib)
	uib.Status.LastClickedAt = metav1.NowMicro()
	uib.Status.Inputs = []v1alpha1.UIInputStatus{
		{
			Name:   actionUIInputName,
			Hidden: &v1alpha1.UIHiddenInputStatus{Value: "fdasfsa"},
		},
	}
	err := f.Client.Status().Update(f.ctx, &uib)
	require.NoError(t, err)

	f.MustReconcile(tbName)

	tb := f.toggleButton()
	require.Contains(t, tb.Status.Error, "unexpected value \"fdasfsa\"")

	f.requireSteadyState()
}

func TestReconciler_noConfigMap(t *testing.T) {
	f := newFixture(t)

	f.createToggleButton()

	tb := f.toggleButton()
	require.Equal(t, "no such ConfigMap \"toggle-cm\"", tb.Status.Error)

	f.requireSteadyState()
}

func TestReconciler_doesntSpecifyConfigMap(t *testing.T) {
	f := newFixture(t)

	f.createToggleButton()
	tb := f.toggleButton()
	tb.Spec.StateSource.ConfigMap = nil
	f.Update(&tb)

	// simulate a click on the button, but with an unknown value
	uib := f.uiButton()
	require.NotNil(t, uib)
	uib.Status.LastClickedAt = metav1.NowMicro()
	uib.Status.Inputs = []v1alpha1.UIInputStatus{
		{
			Name:   actionUIInputName,
			Hidden: &v1alpha1.UIHiddenInputStatus{Value: "foo"},
		},
	}
	err := f.Client.Status().Update(f.ctx, &uib)
	require.NoError(t, err)

	f.MustReconcile(tbName)

	tb = f.toggleButton()
	require.Contains(t, tb.Status.Error, "Spec.StateSource.ConfigMap is nil")

	f.requireSteadyState()
}

func TestReconciler_ConfigMapUnexpectedValue(t *testing.T) {
	f := newFixture(t)
	f.setupTest()

	cm := f.configMap()
	tb := f.toggleButton()
	cm.Data[tb.Spec.StateSource.ConfigMap.Key] = "asdf"
	err := f.Client.Update(f.ctx, &cm)
	require.NoError(t, err)

	f.MustReconcile(tbName)

	tb = f.toggleButton()
	require.Contains(t, tb.Status.Error, "unknown value \"asdf\"")

	f.requireSteadyState()
}

func TestReconciler_clearError(t *testing.T) {
	f := newFixture(t)
	f.setupTest()

	cm := f.configMap()
	tb := f.toggleButton()
	cm.Data[tb.Spec.StateSource.ConfigMap.Key] = "asdf"
	err := f.Client.Status().Update(f.ctx, &cm)
	require.NoError(t, err)
	f.MustReconcile(tbName)

	cm.Data[tb.Spec.StateSource.ConfigMap.Key] = tb.Spec.StateSource.ConfigMap.OnValue
	err = f.Client.Status().Update(f.ctx, &cm)
	require.NoError(t, err)
	f.MustReconcile(tbName)

	tb = f.toggleButton()
	require.Equal(t, "", tb.Status.Error)

	f.requireSteadyState()
}

type fixture struct {
	*fake.ControllerFixture
	t   *testing.T
	ctx context.Context
}

func newFixture(t *testing.T) *fixture {
	cfb := fake.NewControllerFixtureBuilder(t)
	r := NewReconciler(cfb.Client, cfb.Client.Scheme())
	return &fixture{
		ControllerFixture: cfb.Build(r),
		t:                 t,
		ctx:               context.Background(),
	}
}

func (f *fixture) createConfigMap() {
	err := f.Client.Create(f.ctx, &v1alpha1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "toggle-cm",
		},
		Data: map[string]string{
			"enabled": "false",
		},
	})
	require.NoError(f.t, err)
}

func (f *fixture) createToggleButton() {
	err := f.Client.Create(f.ctx, &v1alpha1.ToggleButton{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tbName.Name,
			Namespace: tbName.Namespace,
		},
		Spec: v1alpha1.ToggleButtonSpec{
			On: v1alpha1.ToggleButtonStateSpec{
				Text:     "disable",
				IconName: "stop",
			},
			Off: v1alpha1.ToggleButtonStateSpec{
				Text:     "enable",
				IconName: "play_arrow",
			},
			DefaultOn: false,
			StateSource: v1alpha1.StateSource{
				ConfigMap: &v1alpha1.ConfigMapStateSource{
					Name:     "toggle-cm",
					Key:      "enabled",
					OnValue:  "true",
					OffValue: "false",
				},
			},
		},
	})
	require.NoError(f.t, err)
	f.MustReconcile(tbName)
}

func (f *fixture) setupTest() {
	f.createConfigMap()
	f.createToggleButton()
}

func (f *fixture) toggleButton() v1alpha1.ToggleButton {
	var tb v1alpha1.ToggleButton
	f.MustGet(tbName, &tb)
	return tb
}

func (f *fixture) uiButton() v1alpha1.UIButton {
	var uib v1alpha1.UIButton
	f.MustGet(types.NamespacedName{Name: "toggle-test-toggle"}, &uib)
	return uib
}

func (f *fixture) configMap() v1alpha1.ConfigMap {
	var cm v1alpha1.ConfigMap
	f.MustGet(types.NamespacedName{Name: "toggle-cm"}, &cm)
	return cm
}

// ensures a reconcile doesn't cause any object changes and thus risk infinitely reconciling
func (f *fixture) requireSteadyState() {
	tb := f.toggleButton()
	f.MustReconcile(tbName)
	tb2 := f.toggleButton()
	require.Equal(f.t, tb.ObjectMeta.ResourceVersion, tb2.ObjectMeta.ResourceVersion)
}

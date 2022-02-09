package trigger

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestExtractKeysForIndexer(t *testing.T) {
	const ns = "fake-ns"

	key := func(name string, kind string) indexer.Key {
		return indexer.Key{
			Name: types.NamespacedName{Namespace: ns, Name: name},
			GVK: schema.GroupVersionKind{
				Group:   "tilt.dev",
				Version: "v1alpha1",
				Kind:    kind,
			},
		}
	}

	fwKey := func(name string) indexer.Key {
		return key(name, "FileWatch")
	}

	btnKey := func(name string) indexer.Key {
		return key(name, "UIButton")
	}

	type tc struct {
		specs    TriggerSpecs
		expected []indexer.Key
	}

	tcs := []tc{
		{TriggerSpecs{}, []indexer.Key(nil)},
		{
			TriggerSpecs{RestartOn: &v1alpha1.RestartOnSpec{FileWatches: []string{"foo"}}},
			[]indexer.Key{fwKey("foo")},
		},
		{
			TriggerSpecs{StartOn: &v1alpha1.StartOnSpec{UIButtons: []string{"btn"}}},
			[]indexer.Key{btnKey("btn")},
		},
		{
			TriggerSpecs{StopOn: &v1alpha1.StopOnSpec{UIButtons: []string{"btn"}}},
			[]indexer.Key{btnKey("btn")},
		}, {
			TriggerSpecs{
				RestartOn: &v1alpha1.RestartOnSpec{FileWatches: []string{"foo"}, UIButtons: []string{"bar"}},
				StartOn:   &v1alpha1.StartOnSpec{UIButtons: []string{"baz"}},
				StopOn:    &v1alpha1.StopOnSpec{UIButtons: []string{"quu"}},
			},
			[]indexer.Key{fwKey("foo"), btnKey("bar"), btnKey("baz"), btnKey("quu")},
		},
	}

	for _, tc := range tcs {
		keys := extractKeysForIndexer(ns, tc.specs)
		assert.ElementsMatchf(t, tc.expected, keys,
			"Indexer keys did not match\nRestartOnSpec: %s\nStartOnSpec: %s\nCancelSpec: %s\n",
			strings.TrimSpace(spew.Sdump(tc.specs.RestartOn)),
			spew.Sdump(tc.specs.StartOn),
			spew.Sdump(tc.specs.StopOn))
	}
}

func TestFetchObjects(t *testing.T) {
	f := fake.NewControllerFixtureBuilder(t).Build(noopController{})

	f.Create(&v1alpha1.FileWatch{ObjectMeta: metav1.ObjectMeta{Name: "fw1"}})
	f.Create(&v1alpha1.FileWatch{ObjectMeta: metav1.ObjectMeta{Name: "fw2"}})
	f.Create(&v1alpha1.UIButton{ObjectMeta: metav1.ObjectMeta{Name: "btn1"}})
	f.Create(&v1alpha1.UIButton{ObjectMeta: metav1.ObjectMeta{Name: "btn2"}})
	f.Create(&v1alpha1.UIButton{ObjectMeta: metav1.ObjectMeta{Name: "btn4"}})

	triggerObjs, err := FetchObjects(f.Context(), f.Client,
		TriggerSpecs{
			RestartOn: &v1alpha1.RestartOnSpec{
				FileWatches: []string{"fw1", "fw2", "fw3"},
				UIButtons:   []string{"btn1"},
			},
			StartOn: &v1alpha1.StartOnSpec{
				UIButtons: []string{"btn2", "btn3"},
			},
			StopOn: &v1alpha1.StopOnSpec{
				UIButtons: []string{"btn4", "btn5"},
			},
		})

	require.NoError(t, err)
	assert.NotNil(t, triggerObjs.FileWatches["fw1"])
	assert.NotNil(t, triggerObjs.FileWatches["fw2"])
	// fw3 doesn't exist but should have been silently ignored
	assert.Nil(t, triggerObjs.FileWatches["fw3"])

	assert.NotNil(t, triggerObjs.UIButtons["btn1"])
	assert.NotNil(t, triggerObjs.UIButtons["btn2"])
	// btn3 doesn't exist but should have been silently ignored
	assert.Nil(t, triggerObjs.UIButtons["btn3"])

	assert.NotNil(t, triggerObjs.UIButtons["btn4"])
	// btn5 doesn't exist but should have been silently ignored
	assert.Nil(t, triggerObjs.UIButtons["btn5"])
}

func TestFetchObjects_Error(t *testing.T) {
	cli := &explodingReader{err: errors.New("oh no")}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	triggerObjs, err := FetchObjects(ctx, cli, TriggerSpecs{RestartOn: &v1alpha1.RestartOnSpec{FileWatches: []string{"fw"}}})
	require.Error(t, err, "FetchObjects should have failed with an error")
	require.Empty(t, triggerObjs.FileWatches)
	require.Empty(t, triggerObjs.UIButtons)
}

type noopController struct{}

func (n noopController) CreateBuilder(_ ctrl.Manager) (*builder.Builder, error) {
	return nil, nil
}

func (n noopController) Reconcile(_ context.Context, _ reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

type explodingReader struct {
	err error
}

func (e explodingReader) Get(_ context.Context, _ ctrlclient.ObjectKey, _ ctrlclient.Object) error {
	return e.err
}

func (e explodingReader) List(_ context.Context, _ ctrlclient.ObjectList, _ ...ctrlclient.ListOption) error {
	return e.err
}

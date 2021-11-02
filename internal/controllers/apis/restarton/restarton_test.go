package restarton

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
		restartOn *v1alpha1.RestartOnSpec
		startOn   *v1alpha1.StartOnSpec
		expected  []indexer.Key
	}

	tcs := []tc{
		{nil, nil, []indexer.Key(nil)},
		{
			&v1alpha1.RestartOnSpec{FileWatches: []string{"foo"}},
			nil,
			[]indexer.Key{fwKey("foo")},
		},
		{
			nil,
			&v1alpha1.StartOnSpec{UIButtons: []string{"btn"}},
			[]indexer.Key{btnKey("btn")},
		},
		{
			&v1alpha1.RestartOnSpec{FileWatches: []string{"foo"}, UIButtons: []string{"bar"}},
			&v1alpha1.StartOnSpec{UIButtons: []string{"baz"}},
			[]indexer.Key{fwKey("foo"), btnKey("bar"), btnKey("baz")},
		},
	}

	for _, tc := range tcs {
		keys := ExtractKeysForIndexer(ns, tc.restartOn, tc.startOn)
		assert.ElementsMatchf(t, tc.expected, keys,
			"Indexer keys did not match\nRestartOnSpec: %s\nStartOnSpec: %s",
			strings.TrimSpace(spew.Sdump(tc.restartOn)),
			spew.Sdump(tc.startOn))
	}
}

func TestFetchObjects(t *testing.T) {
	f := fake.NewControllerFixtureBuilder(t).Build(noopController{})

	f.Create(&v1alpha1.FileWatch{ObjectMeta: metav1.ObjectMeta{Name: "fw1"}})
	f.Create(&v1alpha1.FileWatch{ObjectMeta: metav1.ObjectMeta{Name: "fw2"}})
	f.Create(&v1alpha1.UIButton{ObjectMeta: metav1.ObjectMeta{Name: "btn1"}})
	f.Create(&v1alpha1.UIButton{ObjectMeta: metav1.ObjectMeta{Name: "btn2"}})

	restartObjs, err := FetchObjects(f.Context(), f.Client,
		&v1alpha1.RestartOnSpec{
			FileWatches: []string{"fw1", "fw2", "fw3"},
			UIButtons:   []string{"btn1"},
		},
		&v1alpha1.StartOnSpec{
			UIButtons: []string{"btn2", "btn3"},
		})
	require.NoError(t, err)
	assert.NotNil(t, restartObjs.FileWatches["fw1"])
	assert.NotNil(t, restartObjs.FileWatches["fw2"])
	// fw3 doesn't exist but should have been silently ignored
	assert.Nil(t, restartObjs.FileWatches["fw3"])

	assert.NotNil(t, restartObjs.UIButtons["btn1"])
	assert.NotNil(t, restartObjs.UIButtons["btn2"])
	// btn3 doesn't exist but should have been silently ignored
	assert.Nil(t, restartObjs.UIButtons["btn3"])
}

func TestFetchObjects_Error(t *testing.T) {
	cli := &explodingReader{err: errors.New("oh no")}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	restartObjs, err := FetchObjects(ctx, cli, &v1alpha1.RestartOnSpec{FileWatches: []string{"fw"}}, nil)
	require.Error(t, err, "FetchObjects should have failed with an error")
	require.Empty(t, restartObjs.FileWatches)
	require.Empty(t, restartObjs.UIButtons)
}

type noopController struct{}

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

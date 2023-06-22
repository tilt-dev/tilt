package trigger

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/controllers/apiset"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/timecmp"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestSetupControllerRestartOn(t *testing.T) {
	cfb := fake.NewControllerFixtureBuilder(t)

	spec := &v1alpha1.RestartOnSpec{
		UIButtons:   []string{"btn1"},
		FileWatches: []string{"fw1"},
	}

	c := &fakeReconciler{
		indexer: indexer.NewIndexer(cfb.Scheme()),
		onCreateBuilder: func(b *builder.Builder, i *indexer.Indexer) {
			b.For(&v1alpha1.Cmd{})
			SetupControllerRestartOn(b, i, func(_ ctrlclient.Object) *v1alpha1.RestartOnSpec {
				return spec
			})
		},
	}
	f := cfb.Build(c)

	cmd := &v1alpha1.Cmd{ObjectMeta: metav1.ObjectMeta{Name: "cmd1"}}
	f.Create(cmd)
	c.indexer.OnReconcile(types.NamespacedName{Name: cmd.Name}, cmd)

	ctx := context.Background()
	reqs := c.indexer.Enqueue(ctx, &v1alpha1.UIButton{ObjectMeta: metav1.ObjectMeta{Name: "btn1"}})
	require.Equal(t, []reconcile.Request{{NamespacedName: types.NamespacedName{Name: "cmd1"}}}, reqs)

	reqs = c.indexer.Enqueue(ctx, &v1alpha1.FileWatch{ObjectMeta: metav1.ObjectMeta{Name: "fw1"}})
	require.Equal(t, []reconcile.Request{{NamespacedName: types.NamespacedName{Name: "cmd1"}}}, reqs)

	// fw named btn1, which doesn't exist
	reqs = c.indexer.Enqueue(ctx, &v1alpha1.FileWatch{ObjectMeta: metav1.ObjectMeta{Name: "btn1"}})
	require.Len(t, reqs, 0)
}

func TestSetupControllerStartOn(t *testing.T) {
	ctx := context.Background()
	cfb := fake.NewControllerFixtureBuilder(t)

	spec := &v1alpha1.StartOnSpec{
		UIButtons: []string{"btn1"},
	}

	c := &fakeReconciler{
		indexer: indexer.NewIndexer(cfb.Scheme()),
		onCreateBuilder: func(b *builder.Builder, i *indexer.Indexer) {
			b.For(&v1alpha1.Cmd{})
			SetupControllerStartOn(b, i, func(_ ctrlclient.Object) *v1alpha1.StartOnSpec {
				return spec
			})
		},
	}
	f := cfb.Build(c)

	cmd := &v1alpha1.Cmd{ObjectMeta: metav1.ObjectMeta{Name: "cmd1"}}
	f.Create(cmd)
	c.indexer.OnReconcile(types.NamespacedName{Name: cmd.Name}, cmd)

	reqs := c.indexer.Enqueue(ctx, &v1alpha1.UIButton{ObjectMeta: metav1.ObjectMeta{Name: "btn1"}})
	require.Equal(t, []reconcile.Request{{NamespacedName: types.NamespacedName{Name: "cmd1"}}}, reqs)

	// wrong name
	reqs = c.indexer.Enqueue(ctx, &v1alpha1.UIButton{ObjectMeta: metav1.ObjectMeta{Name: "btn2"}})
	require.Len(t, reqs, 0)

	// wrong type
	reqs = c.indexer.Enqueue(ctx, &v1alpha1.FileWatch{ObjectMeta: metav1.ObjectMeta{Name: "btn1"}})
	require.Len(t, reqs, 0)
}

func TestSetupControllerStopOn(t *testing.T) {
	cfb := fake.NewControllerFixtureBuilder(t)

	spec := &v1alpha1.StopOnSpec{
		UIButtons: []string{"btn1"},
	}

	c := &fakeReconciler{
		indexer: indexer.NewIndexer(cfb.Scheme()),
		onCreateBuilder: func(b *builder.Builder, i *indexer.Indexer) {
			b.For(&v1alpha1.Cmd{})
			SetupControllerStopOn(b, i, func(_ ctrlclient.Object) *v1alpha1.StopOnSpec {
				return spec
			})
		},
	}
	f := cfb.Build(c)

	cmd := &v1alpha1.Cmd{ObjectMeta: metav1.ObjectMeta{Name: "cmd1"}}
	f.Create(cmd)
	c.indexer.OnReconcile(types.NamespacedName{Name: cmd.Name}, cmd)

	ctx := context.Background()
	reqs := c.indexer.Enqueue(ctx, &v1alpha1.UIButton{ObjectMeta: metav1.ObjectMeta{Name: "btn1"}})
	require.Equal(t, []reconcile.Request{{NamespacedName: types.NamespacedName{Name: "cmd1"}}}, reqs)

	// wrong name
	reqs = c.indexer.Enqueue(ctx, &v1alpha1.UIButton{ObjectMeta: metav1.ObjectMeta{Name: "btn2"}})
	require.Len(t, reqs, 0)

	// wrong type
	reqs = c.indexer.Enqueue(ctx, &v1alpha1.FileWatch{ObjectMeta: metav1.ObjectMeta{Name: "btn1"}})
	require.Len(t, reqs, 0)
}

func TestLastRestartEvent(t *testing.T) {
	ctx := context.Background()

	objs := make(apiset.ObjectSet)
	btns := []*v1alpha1.UIButton{
		button("btn10", time.Unix(10, 0)),
		button("btn0", time.Unix(0, 0)),
		button("btn3", time.Unix(3, 0)),
	}
	for _, btn := range btns {
		objs.Add(btn)
	}

	fws := []*v1alpha1.FileWatch{
		filewatch("fw2", time.Unix(2, 0)),
		filewatch("fw20", time.Unix(20, 0)),
		filewatch("fw7", time.Unix(7, 0)),
	}
	for _, fw := range fws {
		objs.Add(fw)
	}

	r := &fakeReader{objs: objs}

	for _, tc := range []struct {
		name           string
		fws            []string
		buttons        []string
		expectedButton string
		expectedFws    []string
		expectedTime   time.Time
	}{
		{"no match", nil, nil, "", nil, time.Time{}},
		{"one fw", []string{"fw2"}, nil, "", []string{"fw2"}, time.Unix(2, 0)},
		{"one button", nil, []string{"btn3"}, "btn3", nil, time.Unix(3, 0)},
		{"all buttons", nil, []string{"btn10", "btn0", "btn3"}, "btn10", nil, time.Unix(10, 0)},
		{"all objects", []string{"fw2", "fw20", "fw7"}, []string{"btn10", "btn0", "btn3"}, "", []string{"fw2", "fw20", "fw7"}, time.Unix(20, 0)},
	} {
		t.Run(tc.name, func(t *testing.T) {

			spec := &v1alpha1.RestartOnSpec{
				FileWatches: tc.fws,
				UIButtons:   tc.buttons,
			}

			ts, btn, fws, err := LastRestartEvent(ctx, r, spec)
			timecmp.RequireTimeEqual(t, tc.expectedTime, ts)
			buttonName := ""
			if btn != nil {
				buttonName = btn.Name
			}
			require.Equal(t, tc.expectedButton, buttonName, "button")
			var fwNames []string
			for _, fw := range fws {
				fwNames = append(fwNames, fw.Name)
			}
			require.ElementsMatch(t, tc.expectedFws, fwNames, "fws")
			require.NoError(t, err, "err")
		})
	}
}

func TestLastStartEvent(t *testing.T) {
	ctx := context.Background()

	objs := make(apiset.ObjectSet)
	btns := []*v1alpha1.UIButton{
		button("btn10", time.Unix(10, 0)),
		button("btn0", time.Unix(0, 0)),
		button("btn3", time.Unix(3, 0)),
	}
	for _, btn := range btns {
		objs.Add(btn)
	}

	r := &fakeReader{objs: objs}

	for _, tc := range []struct {
		name           string
		buttons        []string
		expectedButton string
		expectedTime   time.Time
	}{
		{"no match", nil, "", time.Time{}},
		{"one button", []string{"btn3"}, "btn3", time.Unix(3, 0)},
		{"all buttons", []string{"btn10", "btn0", "btn3"}, "btn10", time.Unix(10, 0)},
	} {
		t.Run(tc.name, func(t *testing.T) {

			spec := &v1alpha1.StartOnSpec{
				UIButtons: tc.buttons,
			}

			ts, btn, err := LastStartEvent(ctx, r, spec)
			require.Equal(t, tc.expectedTime.UTC(), ts.UTC(), "timestamp")
			buttonName := ""
			if btn != nil {
				buttonName = btn.Name
			}
			require.Equal(t, tc.expectedButton, buttonName, "button")
			require.NoError(t, err, "err")
		})
	}
}

func TestLastStopEvent(t *testing.T) {
	ctx := context.Background()

	objs := make(apiset.ObjectSet)
	btns := []*v1alpha1.UIButton{
		button("btn10", time.Unix(10, 0)),
		button("btn0", time.Unix(0, 0)),
		button("btn3", time.Unix(3, 0)),
	}
	for _, btn := range btns {
		objs.Add(btn)
	}

	r := &fakeReader{objs: objs}

	for _, tc := range []struct {
		name           string
		buttons        []string
		expectedButton string
		expectedTime   time.Time
	}{
		{"no match", nil, "", time.Time{}},
		{"one button", []string{"btn3"}, "btn3", time.Unix(3, 0)},
		{"all buttons", []string{"btn10", "btn0", "btn3"}, "btn10", time.Unix(10, 0)},
	} {
		t.Run(tc.name, func(t *testing.T) {

			spec := &v1alpha1.StopOnSpec{
				UIButtons: tc.buttons,
			}

			ts, btn, err := LastStopEvent(ctx, r, spec)
			timecmp.RequireTimeEqual(t, tc.expectedTime, ts)
			buttonName := ""
			if btn != nil {
				buttonName = btn.Name
			}
			require.Equal(t, tc.expectedButton, buttonName, "button")
			require.NoError(t, err, "err")
		})
	}
}

func TestLastRestartEventError(t *testing.T) {
	expected := errors.New("oh no")
	cli := &explodingReader{err: expected}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ts, btn, fw, err := LastRestartEvent(ctx, cli, &v1alpha1.RestartOnSpec{FileWatches: []string{"foo"}})
	require.Equal(t, expected, err)
	require.Zero(t, ts, "Timestamp was not zero value")
	require.Nil(t, btn, "Button was not nil")
	require.Nil(t, fw, "FileWatch was not nil")
}

func TestLastStartEventError(t *testing.T) {
	expected := errors.New("oh no")
	cli := &explodingReader{err: expected}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ts, btn, err := LastStartEvent(ctx, cli, &v1alpha1.StartOnSpec{UIButtons: []string{"foo"}})
	require.Equal(t, expected, err)
	require.Zero(t, ts, "Timestamp was not zero value")
	require.Nil(t, btn, "Button was not nil")
}

func TestLastStopEventError(t *testing.T) {
	expected := errors.New("oh no")
	cli := &explodingReader{err: expected}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ts, btn, err := LastStopEvent(ctx, cli, &v1alpha1.StopOnSpec{UIButtons: []string{"foo"}})
	require.Equal(t, expected, err)
	require.Zero(t, ts, "Timestamp was not zero value")
	require.Nil(t, btn, "Button was not nil")
}

type explodingReader struct {
	err error
}

func (e explodingReader) Get(_ context.Context, _ ctrlclient.ObjectKey, _ ctrlclient.Object, _ ...ctrlclient.GetOption) error {
	return e.err
}

func (e explodingReader) List(_ context.Context, _ ctrlclient.ObjectList, _ ...ctrlclient.ListOption) error {
	return e.err
}

type fakeReader struct {
	objs apiset.ObjectSet
}

func (f *fakeReader) Get(ctx context.Context, key ctrlclient.ObjectKey, out ctrlclient.Object, _ ...ctrlclient.GetOption) error {
	if f.objs == nil {
		return errors.New("fakeReader.objs uninitialized")
	}

	typedObjectSet := f.objs.GetSetForType(out.(apiset.Object))
	obj, ok := typedObjectSet[key.Name]
	if !ok {
		return apierrors.NewNotFound(schema.GroupResource{}, key.Name)
	}

	outVal := reflect.ValueOf(out)
	objVal := reflect.ValueOf(obj)
	if !objVal.Type().AssignableTo(outVal.Type()) {
		return fmt.Errorf("fakeReader objs[%s] is type %s, but %s was asked for", key.Name, objVal.Type(), outVal.Type())
	}
	reflect.Indirect(outVal).Set(reflect.Indirect(objVal))
	return nil
}

func (f *fakeReader) List(ctx context.Context, list ctrlclient.ObjectList, opts ...ctrlclient.ListOption) error {
	panic("implement me")
}

var _ ctrlclient.Reader = &fakeReader{}

type fakeReconciler struct {
	indexer         *indexer.Indexer
	onCreateBuilder func(builder *builder.Builder, idxer *indexer.Indexer)
}

func (fr *fakeReconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr)
	fr.onCreateBuilder(b, fr.indexer)

	return b, nil
}

func (fr *fakeReconciler) Reconcile(_ context.Context, _ reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func button(name string, ts time.Time) *v1alpha1.UIButton {
	return &v1alpha1.UIButton{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status:     v1alpha1.UIButtonStatus{LastClickedAt: metav1.NewMicroTime(ts)},
	}
}

func filewatch(name string, ts time.Time) *v1alpha1.FileWatch {
	return &v1alpha1.FileWatch{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status:     v1alpha1.FileWatchStatus{LastEventTime: metav1.NewMicroTime(ts)},
	}
}

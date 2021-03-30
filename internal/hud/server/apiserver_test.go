package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/assets"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Ensure creating objects works with the dynamic API clients.
func TestAPIServerDynamicClient(t *testing.T) {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	memconn := ProvideMemConn()
	cfg, err := ProvideTiltServerOptions(ctx, "localhost", 0, model.TiltBuild{}, memconn)
	require.NoError(t, err)

	hudsc := ProvideHeadsUpServerController(0, cfg, &HeadsUpServer{}, assets.NewFakeServer(), model.WebURL{})
	st := store.NewTestingStore()
	require.NoError(t, hudsc.SetUp(ctx, st))
	defer hudsc.TearDown(ctx)

	// Dynamic type tests
	dynamic, err := ProvideTiltDynamic(cfg)
	require.NoError(t, err)

	specs := map[string]interface{}{
		"FileWatch": map[string]interface{}{
			// this needs to include a valid absolute path for the current GOOS
			"watchedPaths": []string{mustCwd(t)},
		},
		"TiltRun": map[string]interface{}{
			"tiltfilePath":  filepath.Join(mustCwd(t), "Tiltfile"),
			"exitCondition": "manual",
		},
	}

	for _, obj := range v1alpha1.AllResourceObjects() {
		typeName := reflect.TypeOf(obj).Elem().Name()
		t.Run(typeName, func(t *testing.T) {
			objName := fmt.Sprintf("dynamic-%s", strings.ToLower(typeName))
			unstructured := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind":       typeName,
					"apiVersion": v1alpha1.SchemeGroupVersion.String(),
					"metadata": map[string]interface{}{
						"name": objName,
						"annotations": map[string]string{
							"my-random-key": "my-random-value",
						},
					},
					"spec": specs[typeName],
				},
			}

			objClient := dynamic.Resource(obj.GetGroupVersionResource())
			_, err = objClient.Create(ctx, unstructured, metav1.CreateOptions{})
			require.NoError(t, err)

			newObj, err := objClient.Get(ctx, objName, metav1.GetOptions{})
			require.NoError(t, err)

			metadata, err := meta.Accessor(newObj)
			require.NoError(t, err)

			assert.Equal(t, objName, metadata.GetName())
			assert.Equal(t, "my-random-value", metadata.GetAnnotations()["my-random-key"])
		})
	}
}

type typedTestCase struct {
	Name   string
	Create func(ctx context.Context, name string, annotations map[string]string) error
	Get    func(ctx context.Context, name string) (resource.Object, error)
	Watch  func(ctx context.Context) (watch.Interface, error)
}

// Ensure creating objects works with the typed API clients.
func TestAPIServerTypedClient(t *testing.T) {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	memconn := ProvideMemConn()
	cfg, err := ProvideTiltServerOptions(ctx, "localhost", 0, model.TiltBuild{}, memconn)
	require.NoError(t, err)

	hudsc := ProvideHeadsUpServerController(0, cfg, &HeadsUpServer{}, assets.NewFakeServer(), model.WebURL{})
	st := store.NewTestingStore()
	require.NoError(t, hudsc.SetUp(ctx, st))
	defer hudsc.TearDown(ctx)

	clientset, err := ProvideTiltInterface(cfg)
	require.NoError(t, err)

	testCases := []typedTestCase{
		{
			Name: "FileWatch",
			Create: func(ctx context.Context, name string, annotations map[string]string) error {
				_, err := clientset.TiltV1alpha1().FileWatches().Create(ctx, &v1alpha1.FileWatch{
					ObjectMeta: metav1.ObjectMeta{
						Name:        name,
						Annotations: annotations,
					},
					Spec: v1alpha1.FileWatchSpec{
						// this needs to include a valid absolute path for the current GOOS
						WatchedPaths: []string{mustCwd(t)},
					},
				}, metav1.CreateOptions{})
				return err
			},
			Get: func(ctx context.Context, name string) (resource.Object, error) {
				return clientset.TiltV1alpha1().FileWatches().Get(ctx, name, metav1.GetOptions{})
			},
			Watch: func(ctx context.Context) (watch.Interface, error) {
				return clientset.TiltV1alpha1().FileWatches().Watch(ctx, metav1.ListOptions{})
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			objName := fmt.Sprintf("typed-%s", strings.ToLower(testCase.Name))
			watcher, err := testCase.Watch(ctx)
			require.NoError(t, err)
			defer watcher.Stop()

			err = testCase.Create(ctx, objName, map[string]string{
				"my-random-key": "my-random-value",
			})
			require.NoError(t, err)

			newObj, err := testCase.Get(ctx, objName)
			require.NoError(t, err)

			metadata, err := meta.Accessor(newObj)
			require.NoError(t, err)

			assert.Equal(t, objName, metadata.GetName())
			assert.Equal(t, "my-random-value", metadata.GetAnnotations()["my-random-key"])

			watchEvent := <-watcher.ResultChan()
			watchedMetadata, err := meta.Accessor(watchEvent.Object)
			require.NoError(t, err)
			assert.Equal(t, objName, watchedMetadata.GetName())
		})
	}
}

func mustCwd(t testing.TB) string {
	t.Helper()
	cwd, err := os.Getwd()
	require.NoError(t, err, "Could not get current working directory")
	return cwd
}

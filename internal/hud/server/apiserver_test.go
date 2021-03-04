package server

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/akutz/memconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

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
	memconn := &memconn.Provider{}
	cfg, err := ProvideTiltServerOptions(ctx, "localhost", 0, model.TiltBuild{}, memconn)
	require.NoError(t, err)

	hudsc := ProvideHeadsUpServerController(0, cfg, &HeadsUpServer{}, assets.NewFakeServer(), model.WebURL{})
	st := store.NewTestingStore()
	require.NoError(t, hudsc.SetUp(ctx, st))
	defer hudsc.TearDown(ctx)

	// Dynamic type tests
	dynamic, err := ProvideTiltDynamic(cfg)
	require.NoError(t, err)

	// for types with validation logic, a passing spec must be provided here
	sampleSpecs := map[string]map[string]interface{}{
		"FileWatch": {
			"watches": []map[string]interface{}{
				{
					// this needs to be a valid path for the target OS
					"rootPath": mustCwd(t),
					"paths":    []string{"pathToWatch"},
				},
			},
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
					"spec": sampleSpecs[typeName],
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
}

// Ensure creating objects works with the typed API clients.
func TestAPIServerTypedClient(t *testing.T) {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	memconn := &memconn.Provider{}
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
				_, err := clientset.CoreV1alpha1().FileWatches().Create(ctx, &v1alpha1.FileWatch{
					ObjectMeta: metav1.ObjectMeta{
						Name:        name,
						Annotations: annotations,
					},
					Spec: v1alpha1.FileWatchSpec{
						Watches: []v1alpha1.WatchDef{
							{
								// this needs to be a valid path for the target OS
								RootPath: mustCwd(t),
								Paths:    []string{"a path to watch"},
							},
						},
					},
				}, metav1.CreateOptions{})
				return err
			},
			Get: func(ctx context.Context, name string) (resource.Object, error) {
				return clientset.CoreV1alpha1().FileWatches().Get(ctx, name, metav1.GetOptions{})
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			objName := fmt.Sprintf("typed-%s", strings.ToLower(testCase.Name))
			err := testCase.Create(ctx, objName, map[string]string{
				"my-random-key": "my-random-value",
			})
			require.NoError(t, err)

			newObj, err := testCase.Get(ctx, objName)
			require.NoError(t, err)

			metadata, err := meta.Accessor(newObj)
			require.NoError(t, err)

			assert.Equal(t, objName, metadata.GetName())
			assert.Equal(t, "my-random-value", metadata.GetAnnotations()["my-random-key"])
		})
	}
}

func mustCwd(t testing.TB) string {
	dir, err := os.Getwd()
	require.NoError(t, err, "Could not get current working directory")
	return dir
}

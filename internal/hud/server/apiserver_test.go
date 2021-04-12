package server

import (
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

	"github.com/tilt-dev/wmclient/pkg/dirs"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/testdata"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/assets"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Ensure creating objects works with the dynamic API clients.
func TestAPIServerDynamicClient(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()

	dir := dirs.NewTiltDevDirAt(f.Path())
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	memconn := ProvideMemConn()

	cfg, err := ProvideTiltServerOptions(ctx, model.TiltBuild{}, memconn, "corgi-charge", testdata.CertKey(), 0)
	require.NoError(t, err)

	webListener, err := ProvideWebListener("localhost", 0)
	require.NoError(t, err)

	configAccess := ProvideConfigAccess(dir)
	hudsc := ProvideHeadsUpServerController(configAccess, "tilt-default",
		webListener, cfg, &HeadsUpServer{}, assets.NewFakeServer(), model.WebURL{})
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
		"Session": map[string]interface{}{
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

func mustCwd(t testing.TB) string {
	t.Helper()
	cwd, err := os.Getwd()
	require.NoError(t, err, "Could not get current working directory")
	return cwd
}

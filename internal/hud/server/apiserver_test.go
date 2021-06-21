package server

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/apiserver"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/tilt-dev/wmclient/pkg/dirs"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/testdata"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/assets"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Ensure creating objects works with the dynamic API clients.
func TestAPIServerDynamicClient(t *testing.T) {
	f := newAPIServerFixture(t)
	f.start()

	specs := map[string]interface{}{
		"FileWatch": map[string]interface{}{
			// this needs to include a valid absolute path for the current GOOS
			"watchedPaths": []string{mustCwd(t)},
		},
		"Session": map[string]interface{}{
			"tiltfilePath":  filepath.Join(mustCwd(t), "Tiltfile"),
			"exitCondition": "manual",
		},
		"KubernetesDiscovery": map[string]interface{}{
			"watches": []map[string]interface{}{
				{"namespace": "my-namespace", "uid": "my-uid"},
			},
		},
		"KubernetesApply": map[string]interface{}{
			"yaml": testyaml.SanchoYAML,
		},
		"ImageMap": map[string]interface{}{
			"selector": "busybox",
		},
		"UIButton": map[string]interface{}{
			"text": "I'm a button!",
			"location": map[string]interface{}{
				"componentType": "Resource",
				"componentID":   "my-resource",
			},
		},
		"PortForward": map[string]interface{}{
			"podName": "my-pod",
			"forwards": []interface{}{
				map[string]interface{}{
					"localPort":     8080,
					"containerPort": 8000,
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
					"spec": specs[typeName],
				},
			}

			objClient := f.dynamic.Resource(obj.GetGroupVersionResource())
			_, err := objClient.Create(f.ctx, unstructured, metav1.CreateOptions{})
			require.NoError(t, err)

			newObj, err := objClient.Get(f.ctx, objName, metav1.GetOptions{})
			require.NoError(t, err)

			metadata, err := meta.Accessor(newObj)
			require.NoError(t, err)

			assert.Equal(t, objName, metadata.GetName())
			assert.Equal(t, "my-random-value", metadata.GetAnnotations()["my-random-key"])
		})
	}
}

func TestAPIServerProxy(t *testing.T) {
	f := newAPIServerFixture(t)
	f.start()

	reqURL := fmt.Sprintf("http://%s/proxy/apis/tilt.dev/v1alpha1/uibuttons", f.webListener.Addr())
	req, err := http.NewRequestWithContext(f.ctx, http.MethodGet, reqURL, nil)
	require.NoError(t, err, "Failed to create request")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "Request failed")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read response body")
	// don't care about the full body of the response, but it should at least have
	// "kind": "UIButtonList" so look for that as a magic word
	require.Contains(t, string(body), "UIButtonList")
}

func mustCwd(t testing.TB) string {
	t.Helper()
	cwd, err := os.Getwd()
	require.NoError(t, err, "Could not get current working directory")
	return cwd
}

type apiserverFixture struct {
	*tempdir.TempDirFixture
	t               testing.TB
	ctx             context.Context
	conn            apiserver.ConnProvider
	serverConfig    *APIServerConfig
	configAccess    clientcmd.ConfigAccess
	webListener     WebListener
	webListenerHost string
	webListenerPort int
	webURL          model.WebURL
	st              *store.TestingStore
	dynamic         DynamicInterface
}

func newAPIServerFixture(t testing.TB) *apiserverFixture {
	t.Helper()

	tmpdir := tempdir.NewTempDirFixture(t)
	t.Cleanup(tmpdir.TearDown)

	dir := dirs.NewTiltDevDirAt(tmpdir.Path())
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	// since these tests issue network requests, ensure that they don't get stuck perpetually
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	t.Cleanup(cancel)

	memconn := ProvideMemConn()

	cfg, err := ProvideTiltServerOptions(ctx, model.TiltBuild{}, memconn, "corgi-charge", testdata.CertKey(), 0)
	require.NoError(t, err)

	const host = "localhost"
	webListener, err := ProvideWebListener(host, 0)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = webListener.Close()
	})

	webListenerHost, port, err := net.SplitHostPort(webListener.Addr().String())
	require.NoErrorf(t, err, "Invalid listener address: %s", webListener.Addr().String())
	webListenerPort, err := strconv.Atoi(port)
	require.NoErrorf(t, err, "Invalid listener port: %s", port)
	webURL, err := url.Parse(fmt.Sprintf("http://%s:%s/", host, port))
	require.NoError(t, err, "Unable to create WebURL")

	configAccess := ProvideConfigAccess(dir)

	// Dynamic type tests
	dynamic, err := ProvideTiltDynamic(cfg)
	require.NoError(t, err)

	f := &apiserverFixture{
		TempDirFixture:  tmpdir,
		t:               t,
		ctx:             ctx,
		conn:            memconn,
		serverConfig:    cfg,
		configAccess:    configAccess,
		webListener:     webListener,
		webListenerHost: webListenerHost,
		webListenerPort: webListenerPort,
		webURL:          model.WebURL(*webURL),
		st:              store.NewTestingStore(),
		dynamic:         dynamic,
	}
	return f
}

func (f *apiserverFixture) start() *HeadsUpServerController {
	f.t.Helper()
	hudsc := ProvideHeadsUpServerController(f.configAccess, "tilt-default",
		f.webListener, f.serverConfig, &HeadsUpServer{}, assets.NewFakeServer(), f.webURL)
	require.NoError(f.t, hudsc.SetUp(f.ctx, f.st))
	f.t.Cleanup(func() {
		hudsc.TearDown(f.ctx)
	})
	return hudsc
}

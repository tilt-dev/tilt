package k8s

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/kube"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/resource"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	restfake "k8s.io/client-go/rest/fake"
	ktesting "k8s.io/client-go/testing"

	"github.com/tilt-dev/clusterid"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/testutils"
)

func TestEmptyNamespace(t *testing.T) {
	var emptyNamespace Namespace
	assert.True(t, emptyNamespace.Empty())
	assert.True(t, emptyNamespace == "")
	assert.Equal(t, "default", emptyNamespace.String())
}

func TestNotEmptyNamespace(t *testing.T) {
	var ns Namespace = "x"
	assert.False(t, ns.Empty())
	assert.False(t, ns == "")
	assert.Equal(t, "x", ns.String())
}

func TestUpsert(t *testing.T) {
	f := newClientTestFixture(t)
	postgres, err := ParseYAMLFromString(testyaml.PostgresYAML)
	assert.Nil(t, err)
	_, err = f.k8sUpsert(f.ctx, postgres)
	assert.Nil(t, err)
	assert.Equal(t, 5, len(f.resourceClient.updates))
}

func TestDelete(t *testing.T) {
	f := newClientTestFixture(t)
	postgres, err := ParseYAMLFromString(testyaml.PostgresYAML)
	assert.Nil(t, err)
	err = f.client.Delete(f.ctx, postgres, time.Minute)
	assert.Nil(t, err)
	assert.Equal(t, 5, len(f.resourceClient.deletes))
}

func TestDeleteMissingKind(t *testing.T) {
	f := newClientTestFixture(t)
	f.resourceClient.buildErrFn = func(e K8sEntity) error {
		if e.GVK().Kind == "StatefulSet" {
			return fmt.Errorf(`no matches for kind "StatefulSet" in version "apps/v1"`)
		}
		return nil
	}

	postgres, err := ParseYAMLFromString(testyaml.PostgresYAML)
	assert.Nil(t, err)
	err = f.client.Delete(f.ctx, postgres, time.Minute)
	assert.Nil(t, err)
	assert.Equal(t, 4, len(f.resourceClient.deletes))

	kinds := []string{}
	for _, r := range f.resourceClient.deletes {
		kinds = append(kinds, r.Object.GetObjectKind().GroupVersionKind().Kind)
	}
	assert.Equal(t,
		[]string{"ConfigMap", "PersistentVolume", "PersistentVolumeClaim", "Service"},
		kinds)
}

func TestUpsertAnnotationTooLong(t *testing.T) {
	postgres := MustParseYAMLFromString(t, testyaml.PostgresYAML)

	// These error strings are from the Kubernetes API server which can have
	// different error messages depending on the version.
	//
	// The error string was changed in K8S v1.32:
	// See: https://github.com/kubernetes/kubernetes/pull/128553
	errorMsgs := []string{
		`The ConfigMap "postgres-config" is invalid: metadata.annotations: Too long: must have at most 262144 bytes`,
		`The ConfigMap "postgres-config" is invalid: metadata.annotations: Too long: may not be more than 262144 bytes`,
	}

	for _, errorMsg := range errorMsgs {
		t.Run(errorMsg, func(t *testing.T) {
			f := newClientTestFixture(t)

			f.resourceClient.updateErr = errors.New(errorMsg)
			_, err := f.k8sUpsert(f.ctx, postgres)
			assert.Nil(t, err)
			assert.Equal(t, 0, len(f.resourceClient.creates))
			assert.Equal(t, 1, len(f.resourceClient.createOrReplaces))
			assert.Equal(t, 4, len(f.resourceClient.updates))
		})
	}
}

func TestUpsert413(t *testing.T) {
	f := newClientTestFixture(t)
	postgres := MustParseYAMLFromString(t, testyaml.PostgresYAML)

	f.resourceClient.updateErr = fmt.Errorf(`the server responded with the status code 413 but did not return more information (post customresourcedefinitions.apiextensions.k8s.io)`)
	_, err := f.k8sUpsert(f.ctx, postgres)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(f.resourceClient.creates))
	assert.Equal(t, 1, len(f.resourceClient.createOrReplaces))
	assert.Equal(t, 4, len(f.resourceClient.updates))
}

func TestUpsert413Structured(t *testing.T) {
	f := newClientTestFixture(t)
	postgres := MustParseYAMLFromString(t, testyaml.PostgresYAML)

	f.resourceClient.updateErr = &apierrors.StatusError{
		ErrStatus: metav1.Status{
			Message: "too large",
			Reason:  "TooLarge",
			Code:    http.StatusRequestEntityTooLarge,
		},
	}
	_, err := f.k8sUpsert(f.ctx, postgres)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(f.resourceClient.creates))
	assert.Equal(t, 1, len(f.resourceClient.createOrReplaces))
	assert.Equal(t, 4, len(f.resourceClient.updates))
}

func TestUpsertStatefulsetForbidden(t *testing.T) {
	f := newClientTestFixture(t)
	postgres, err := ParseYAMLFromString(testyaml.PostgresYAML)
	assert.Nil(t, err)

	f.resourceClient.updateErr = fmt.Errorf(`The StatefulSet "postgres" is invalid: spec: Forbidden: updates to statefulset spec for fields other than 'replicas', 'template', and 'updateStrategy' are forbidden.`)
	_, err = f.k8sUpsert(f.ctx, postgres)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(f.resourceClient.creates))
	assert.Equal(t, 4, len(f.resourceClient.updates))
}

func TestUpsertToTerminatingNamespaceForbidden(t *testing.T) {
	f := newClientTestFixture(t)
	postgres, err := ParseYAMLFromString(testyaml.SanchoYAML)
	assert.Nil(t, err)

	// Bad error parsing used to result in us treating this error as an immutable
	// field error. Make sure we treat it as what it is and bail out of `kubectl apply`
	// rather than trying to --force
	errStr := `Error from server (Forbidden): error when creating "STDIN": deployments.apps "sancho" is forbidden: unable to create new content in namespace sancho-ns because it is being terminated`
	f.resourceClient.updateErr = errors.New(errStr)

	_, err = f.k8sUpsert(f.ctx, postgres)
	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), errStr)
	}
	assert.Equal(t, 0, len(f.resourceClient.updates))
	assert.Equal(t, 0, len(f.resourceClient.creates))
}

func TestNodePortServiceURL(t *testing.T) {
	// Taken from a Docker Desktop NodePort service.
	s := &v1.Service{
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeNodePort,
			Ports: []v1.ServicePort{
				{
					NodePort:   31074,
					Port:       9000,
					TargetPort: intstr.FromInt(80),
				},
			},
		},
		Status: v1.ServiceStatus{
			LoadBalancer: v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{Hostname: "localhost"},
				},
			},
		},
	}

	url, err := ServiceURL(s, NodeIP("127.0.0.1"))
	assert.NoError(t, err)
	assert.Equal(t, "http://localhost:31074/", url.String())
}

func TestGetGroup(t *testing.T) {
	for _, test := range []struct {
		name          string
		apiVersion    string
		expectedGroup string
	}{
		{"normal", "apps/v1", "apps"},
		// core types have an empty group
		{"core", "/v1", ""},
		// on some versions of k8s, deployment is buggy and doesn't have a version in its apiVersion
		{"no version", "extensions", "extensions"},
		{"alpha version", "apps/v1alpha1", "apps"},
		{"beta version", "apps/v1beta1", "apps"},
		// I've never seen this in the wild, but the docs say it's legal
		{"alpha version, no second number", "apps/v1alpha", "apps"},
	} {
		t.Run(test.name, func(t *testing.T) {
			obj := v1.ObjectReference{APIVersion: test.apiVersion}
			assert.Equal(t, test.expectedGroup, ReferenceGVK(obj).Group)
		})
	}
}

func TestServerHealth(t *testing.T) {
	// NOTE: the health endpoint contract only specifies that 200 is healthy
	// 	and any other status code indicates not-healthy; in practice, apiserver
	// 	seems to always use 500, but we throw a couple different status codes
	// 	in here in the spirit of the contract
	// 	see https://github.com/kubernetes/kubernetes/blob/9918aa1e035a00bc7c0f16a05e1b222650b3eabc/staging/src/k8s.io/apiserver/pkg/server/healthz/healthz.go#L258
	for _, tc := range []struct {
		name            string
		liveErr         error
		liveStatusCode  int
		readyErr        error
		readyStatusCode int
	}{
		{name: "Healthy", liveStatusCode: http.StatusOK, readyStatusCode: http.StatusOK},
		{name: "NotLive", liveStatusCode: http.StatusServiceUnavailable, readyStatusCode: http.StatusServiceUnavailable},
		{name: "NotReady", liveStatusCode: http.StatusOK, readyStatusCode: http.StatusInternalServerError},
		{name: "ErrorLivez", liveErr: errors.New("fake livez network error")},
		{name: "ErrorReadyz", liveStatusCode: http.StatusOK, readyErr: errors.New("fake readyz network error")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newClientTestFixture(t)
			f.restClient.Client = restfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/livez":
					if tc.liveErr != nil {
						return nil, tc.liveErr
					}
					return &http.Response{
						StatusCode: tc.liveStatusCode,
						Body:       io.NopCloser(strings.NewReader("fake livez response")),
					}, nil
				case "/readyz":
					if tc.readyErr != nil {
						return nil, tc.readyErr
					}
					return &http.Response{
						StatusCode: tc.readyStatusCode,
						Body:       io.NopCloser(strings.NewReader("fake readyz response")),
					}, nil
				}
				err := fmt.Errorf("unsupported request: %s", req.URL.Path)
				t.Fatal(err.Error())
				return nil, err
			})

			health, err := f.client.ClusterHealth(f.ctx, true)
			if tc.liveErr != nil {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.liveErr.Error(),
					"livez error did not match")
				return
			} else if tc.readyErr != nil {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.readyErr.Error(),
					"readyz error did not match")
				return
			} else {
				require.NoError(t, err)
			}

			// verbose output is only checked on success - some of the standard
			// error handling in the K8s helpers massages non-200 requests, so
			// it's too brittle to check against
			isLive := tc.liveStatusCode == http.StatusOK
			if assert.Equal(t, isLive, health.Live, "livez") && isLive {
				assert.Equal(t, "fake livez response", health.LiveOutput)
			}
			isReady := tc.readyStatusCode == http.StatusOK
			if assert.Equal(t, isReady, health.Ready, "readyz") && isReady {
				assert.Equal(t, "fake readyz response", health.ReadyOutput)
			}
		})
	}

}

type fakeResourceClient struct {
	updates          kube.ResourceList
	creates          kube.ResourceList
	deletes          kube.ResourceList
	createOrReplaces kube.ResourceList
	updateErr        error
	buildErrFn       func(e K8sEntity) error
}

func (c *fakeResourceClient) Apply(target kube.ResourceList) (*kube.Result, error) {
	defer func() {
		c.updateErr = nil
	}()

	if c.updateErr != nil {
		return nil, c.updateErr
	}
	c.updates = append(c.updates, target...)
	return &kube.Result{Updated: target}, nil
}
func (c *fakeResourceClient) Delete(l kube.ResourceList) (*kube.Result, []error) {
	c.deletes = append(c.deletes, l...)
	return &kube.Result{Deleted: l}, nil
}
func (c *fakeResourceClient) Create(l kube.ResourceList) (*kube.Result, error) {
	c.creates = append(c.creates, l...)
	return &kube.Result{Created: l}, nil
}
func (c *fakeResourceClient) CreateOrReplace(l kube.ResourceList) (*kube.Result, error) {
	c.createOrReplaces = append(c.createOrReplaces, l...)
	return &kube.Result{Updated: l}, nil
}
func (c *fakeResourceClient) Build(r io.Reader, validate bool) (kube.ResourceList, error) {
	entities, err := ParseYAML(r)
	if err != nil {
		return nil, err
	}
	list := kube.ResourceList{}
	for _, e := range entities {
		if c.buildErrFn != nil {
			err := c.buildErrFn(e)
			if err != nil {
				// Stop processing further resources.
				//
				// NOTE(nick): The real client behavior is more complex than this,
				// where sometimes it seems to continue and other times it doesn't,
				// but we want our code to handle "worst" case conditions.
				return list, err
			}
		}

		list = append(list, &resource.Info{
			// Create a fake HTTP client that returns 404 for every request.
			Client: &restfake.RESTClient{
				NegotiatedSerializer: scheme.Codecs,
				Resp:                 &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(&bytes.Buffer{})},
			},
			Mapping:   &meta.RESTMapping{Scope: meta.RESTScopeNamespace},
			Object:    e.Obj,
			Name:      e.Name(),
			Namespace: string(e.Namespace()),
		})
	}

	return list, nil
}

type clientTestFixture struct {
	t              *testing.T
	ctx            context.Context
	client         K8sClient
	tracker        ktesting.ObjectTracker
	watchNotify    chan watch.Interface
	resourceClient *fakeResourceClient
	restClient     *restfake.RESTClient
}

func newClientTestFixture(t *testing.T) *clientTestFixture {
	ret := &clientTestFixture{}
	ret.t = t
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ret.ctx = ctx

	tracker := ktesting.NewObjectTracker(scheme.Scheme, scheme.Codecs.UniversalDecoder())
	watchNotify := make(chan watch.Interface, 100)
	ret.watchNotify = watchNotify

	cs := &fake.Clientset{}
	cs.AddReactor("*", "*", ktesting.ObjectReaction(tracker))
	cs.AddWatchReactor("*", func(action ktesting.Action) (handled bool, ret watch.Interface, err error) {
		gvr := action.GetResource()
		ns := action.GetNamespace()
		watch, err := tracker.Watch(gvr, ns)
		if err != nil {
			return false, nil, err
		}

		watchNotify <- watch
		return true, watch, nil
	})

	ret.tracker = tracker

	core := cs.CoreV1()
	dc := dynfake.NewSimpleDynamicClient(scheme.Scheme)
	runtimeAsync := newRuntimeAsync(core)
	registryAsync := newRegistryAsync(clusterid.ProductUnknown, core, runtimeAsync)
	resourceClient := &fakeResourceClient{}
	ret.resourceClient = resourceClient

	ret.restClient = &restfake.RESTClient{}

	ret.client = K8sClient{
		product:           clusterid.ProductUnknown,
		core:              core,
		portForwardClient: NewFakePortForwardClient(),
		discovery:         fakeDiscovery{restClient: ret.restClient},
		dynamic:           dc,
		runtimeAsync:      runtimeAsync,
		registryAsync:     registryAsync,
		resourceClient:    resourceClient,
		drm:               fakeRESTMapper{},
	}

	return ret
}

func (c clientTestFixture) k8sUpsert(ctx context.Context, entities []K8sEntity) ([]K8sEntity, error) {
	return c.client.Upsert(ctx, entities, time.Minute)
}

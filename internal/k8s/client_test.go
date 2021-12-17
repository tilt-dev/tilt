package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/kube"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/resource"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	restfake "k8s.io/client-go/rest/fake"
	ktesting "k8s.io/client-go/testing"

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
	err = f.client.Delete(f.ctx, postgres, true)
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
	err = f.client.Delete(f.ctx, postgres, true)
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

func TestUpsertMutableAndImmutable(t *testing.T) {
	f := newClientTestFixture(t)
	eDeploy := MustParseYAMLFromString(t, testyaml.SanchoYAML)[0]
	eJob := MustParseYAMLFromString(t, testyaml.JobYAML)[0]
	eNamespace := MustParseYAMLFromString(t, testyaml.MyNamespaceYAML)[0]

	_, err := f.k8sUpsert(f.ctx, []K8sEntity{eDeploy, eJob, eNamespace})
	if !assert.Nil(t, err) {
		t.FailNow()
	}

	require.Len(t, f.resourceClient.updates, 2)
	require.Len(t, f.resourceClient.creates, 1)

	// compare entities instead of strings because str > entity > string gets weird
	call0Entity := NewK8sEntity(f.resourceClient.updates[0].Object)
	call1Entity := NewK8sEntity(f.resourceClient.updates[1].Object)

	// `apply` should preserve input order of entities (we sort them further upstream)
	require.Equal(t, eDeploy, call0Entity, "expect call 0 to have applied deployment first (preserve input order)")
	require.Equal(t, eNamespace, call1Entity, "expect call 0 to have applied namespace second (preserve input order)")

	call2Entity := NewK8sEntity(f.resourceClient.creates[0].Object)
	require.Equal(t, eJob, call2Entity, "expect create job")
}

func TestUpsertAnnotationTooLong(t *testing.T) {
	f := newClientTestFixture(t)
	postgres := MustParseYAMLFromString(t, testyaml.PostgresYAML)

	f.resourceClient.updateErr = fmt.Errorf(`The ConfigMap "postgres-config" is invalid: metadata.annotations: Too long: must have at most 262144 bytes`)
	_, err := f.k8sUpsert(f.ctx, postgres)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(f.resourceClient.creates))
	assert.Equal(t, 1, len(f.resourceClient.createOrReplaces))
	assert.Equal(t, 4, len(f.resourceClient.updates))
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

	f.resourceClient.updateErr = &errors.StatusError{
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
	f.resourceClient.updateErr = fmt.Errorf(errStr)

	_, err = f.k8sUpsert(f.ctx, postgres)
	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), errStr)
	}
	assert.Equal(t, 0, len(f.resourceClient.updates))
	assert.Equal(t, 0, len(f.resourceClient.creates))
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
				Resp:                 &http.Response{StatusCode: http.StatusNotFound, Body: ioutil.NopCloser(&bytes.Buffer{})},
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
	registryAsync := newRegistryAsync(EnvUnknown, core, runtimeAsync)
	resourceClient := &fakeResourceClient{}
	ret.resourceClient = resourceClient

	ret.client = K8sClient{
		env:               EnvUnknown,
		core:              core,
		portForwardClient: NewFakePortForwardClient(),
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

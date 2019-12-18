package k8s

import (
	"context"
	"fmt"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	ktesting "k8s.io/client-go/testing"

	"github.com/windmilleng/tilt/internal/testutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/k8s/testyaml"
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
	_, err = f.client.Upsert(f.ctx, postgres)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(f.runner.calls))
	assert.Equal(t, []string{"apply", "-o", "yaml", "-f", "-"}, f.runner.calls[0].argv)
}

func TestUpsertMutableAndImmutable(t *testing.T) {
	f := newClientTestFixture(t)
	eDeploy := MustParseYAMLFromString(t, testyaml.SanchoYAML)[0]
	eJob := MustParseYAMLFromString(t, testyaml.JobYAML)[0]
	eNamespace := MustParseYAMLFromString(t, testyaml.MyNamespaceYAML)[0]

	_, err := f.client.Upsert(f.ctx, []K8sEntity{eDeploy, eJob, eNamespace})
	if !assert.Nil(t, err) {
		t.FailNow()
	}

	// two different calls: one for mutable entities (namespace, deployment),
	// one for immutable (job)
	require.Len(t, f.runner.calls, 2)

	call0 := f.runner.calls[0]
	require.Equal(t, []string{"apply", "-o", "yaml", "-f", "-"}, call0.argv, "expected args for call 0")

	// compare entities instead of strings because str > entity > string gets weird
	call0Entities := mustParseYAML(t, call0.stdin)
	require.Len(t, call0Entities, 2, "expect two mutable entities applied")

	// `apply` should preserve input order of entities (we sort them further upstream)
	require.Equal(t, eDeploy, call0Entities[0], "expect call 0 to have applied deployment first (preserve input order)")
	require.Equal(t, eNamespace, call0Entities[1], "expect call 0 to have applied namespace second (preserve input order)")

	call1 := f.runner.calls[1]
	require.Equal(t, []string{"replace", "-o", "yaml", "--force", "-f", "-"}, call1.argv, "expected args for call 1")
	call1Entities := mustParseYAML(t, call1.stdin)
	require.Len(t, call1Entities, 1, "expect only one immutable entity applied")
	require.Equal(t, eJob, call1Entities[0], "expect call 1 to have applied job")
}

func TestUpsertStatefulsetForbidden(t *testing.T) {
	f := newClientTestFixture(t)
	postgres, err := ParseYAMLFromString(testyaml.PostgresYAML)
	assert.Nil(t, err)

	f.setStderr(`The StatefulSet "postgres" is invalid: spec: Forbidden: updates to statefulset spec for fields other than 'replicas', 'template', and 'updateStrategy' are forbidden.`)
	_, err = f.client.Upsert(f.ctx, postgres)
	if assert.Nil(t, err) && assert.Equal(t, 3, len(f.runner.calls)) {
		assert.Equal(t, []string{"apply", "-o", "yaml", "-f", "-"}, f.runner.calls[0].argv)
		assert.Equal(t, []string{"delete", "-f", "-"}, f.runner.calls[1].argv)
		assert.Equal(t, []string{"apply", "-o", "yaml", "-f", "-"}, f.runner.calls[2].argv)
	}
}

func TestUpsertToTerminatingNamespaceForbidden(t *testing.T) {
	f := newClientTestFixture(t)
	postgres, err := ParseYAMLFromString(testyaml.SanchoYAML)
	assert.Nil(t, err)

	// Bad error parsing used to result in us treating this error as an immutable
	// field error. Make sure we treat it as what it is and bail out of `kubectl apply`
	// rather than trying to --force
	errStr := `Error from server (Forbidden): error when creating "STDIN": deployments.apps "sancho" is forbidden: unable to create new content in namespace sancho-ns because it is being terminated`
	f.setStderr(errStr)

	_, err = f.client.Upsert(f.ctx, postgres)
	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), errStr)
	}
	if assert.Equal(t, 1, len(f.runner.calls)) {
		assert.Equal(t, []string{"apply", "-o", "yaml", "-f", "-"}, f.runner.calls[0].argv)
	}

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
			assert.Equal(t, test.expectedGroup, getGroup(obj))
		})
	}
}

type call struct {
	argv  []string
	stdin string
}

type fakeKubectlRunner struct {
	stdout string
	stderr string
	err    error

	calls []call
}

func (f *fakeKubectlRunner) execWithStdin(ctx context.Context, args []string, stdin string) (stdout string, stderr string, err error) {
	f.calls = append(f.calls, call{argv: args, stdin: stdin})

	defer func() {
		f.stdout = ""
		f.stderr = ""
		f.err = nil
	}()
	return f.stdout, f.stderr, f.err
}

func (f *fakeKubectlRunner) exec(ctx context.Context, args []string) (stdout string, stderr string, err error) {
	f.calls = append(f.calls, call{argv: args})
	defer func() {
		f.stdout = ""
		f.stderr = ""
		f.err = nil
	}()
	return f.stdout, f.stderr, f.err
}

var _ kubectlRunner = &fakeKubectlRunner{}

type clientTestFixture struct {
	t           *testing.T
	ctx         context.Context
	client      K8sClient
	runner      *fakeKubectlRunner
	tracker     ktesting.ObjectTracker
	watchNotify chan watch.Interface
}

func newClientTestFixture(t *testing.T) *clientTestFixture {
	ret := &clientTestFixture{}
	ret.t = t
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ret.ctx = ctx
	ret.runner = &fakeKubectlRunner{}

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
	runtimeAsync := newRuntimeAsync(core)
	registryAsync := newRegistryAsync(EnvUnknown, core, runtimeAsync)
	ret.client = K8sClient{
		env:               EnvUnknown,
		kubectlRunner:     ret.runner,
		core:              core,
		portForwardClient: &FakePortForwardClient{},
		runtimeAsync:      runtimeAsync,
		registryAsync:     registryAsync,
	}
	return ret
}

func (c clientTestFixture) addObject(obj runtime.Object) {
	err := c.tracker.Add(obj)
	if err != nil {
		c.t.Fatal(err)
	}
}

func (c clientTestFixture) getPod(id PodID) *v1.Pod {
	c.t.Helper()

	pod, err := c.client.core.Pods(DefaultNamespace.String()).Get(id.String(), metav1.GetOptions{})
	if err != nil {
		c.t.Fatal(err)
	}

	return pod
}

func (c clientTestFixture) updatePod(pod *v1.Pod) {
	gvks, _, err := scheme.Scheme.ObjectKinds(pod)
	if err != nil {
		c.t.Fatalf("updatePod: %v", err)
	} else if len(gvks) == 0 {
		c.t.Fatal("Could not parse pod into k8s schema")
	}
	for _, gvk := range gvks {
		gvr, _ := meta.UnsafeGuessKindToResource(gvk)
		err = c.tracker.Update(gvr, pod, NamespaceFromPod(pod).String())
		if err != nil {
			c.t.Fatal(err)
		}
	}
}

func (c clientTestFixture) setOutput(s string) {
	c.runner.stdout = s
}

func (c clientTestFixture) setStderr(stderr string) {
	c.runner.stderr = stderr
	c.runner.err = fmt.Errorf("exit status 1")
}

func (c clientTestFixture) setError(err error) {
	c.runner.err = err
}

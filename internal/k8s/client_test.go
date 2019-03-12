package k8s

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	apiv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	ktesting "k8s.io/client-go/testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/testutils/output"
)

func TestUpsert(t *testing.T) {
	f := newClientTestFixture(t)
	postgres, err := ParseYAMLFromString(testyaml.PostgresYAML)
	assert.Nil(t, err)
	err = f.client.Upsert(f.ctx, postgres)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(f.runner.calls))
	assert.Equal(t, []string{"apply", "-f", "-"}, f.runner.calls[0].argv)
}

func TestUpsertStatefulsetForbidden(t *testing.T) {
	f := newClientTestFixture(t)
	postgres, err := ParseYAMLFromString(testyaml.PostgresYAML)
	assert.Nil(t, err)

	f.setStderr(`The StatefulSet "postgres" is invalid: spec: Forbidden: updates to statefulset spec for fields other than 'replicas', 'template', and 'updateStrategy' are forbidden.`)
	err = f.client.Upsert(f.ctx, postgres)
	if assert.Nil(t, err) && assert.Equal(t, 3, len(f.runner.calls)) {
		assert.Equal(t, []string{"apply", "-f", "-"}, f.runner.calls[0].argv)
		assert.Equal(t, []string{"delete", "-f", "-"}, f.runner.calls[1].argv)
		assert.Equal(t, []string{"apply", "-f", "-"}, f.runner.calls[2].argv)
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

func (f *fakeKubectlRunner) execWithStdin(ctx context.Context, args []string, stdin io.Reader) (stdout string, stderr string, err error) {
	b, err := ioutil.ReadAll(stdin)
	if err != nil {
		return "", "", errors.Wrap(err, "reading stdin")
	}
	f.calls = append(f.calls, call{argv: args, stdin: string(b)})

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
	ret.ctx = output.CtxForTest()
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
	ret.client = K8sClient{EnvUnknown, ret.runner, core, nil, fakePortForwarder, "", nil, runtimeAsync}
	return ret
}

func (c clientTestFixture) addObject(obj runtime.Object) {
	err := c.tracker.Add(obj)
	if err != nil {
		c.t.Fatal(err)
	}
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

func fakePortForwarder(ctx context.Context, restConfig *rest.Config, core apiv1.CoreV1Interface, namespace string, podID PodID, localPort int, remotePort int) (closer func(), err error) {
	return nil, nil
}

var _ PortForwarder = fakePortForwarder

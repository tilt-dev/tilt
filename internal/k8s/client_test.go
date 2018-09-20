package k8s

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	"k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"

	"github.com/windmilleng/tilt/internal/testutils/output"
)

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
		return "", "", fmt.Errorf("reading stdin: %v", err)
	}
	f.calls = append(f.calls, call{argv: args, stdin: string(b)})
	return f.stdout, f.stderr, f.err
}

func (f *fakeKubectlRunner) exec(ctx context.Context, args []string) (stdout string, stderr string, err error) {
	f.calls = append(f.calls, call{argv: args})
	return f.stdout, f.stderr, f.err
}

var _ kubectlRunner = &fakeKubectlRunner{}

type clientTestFixture struct {
	t      *testing.T
	ctx    context.Context
	client K8sClient
	runner *fakeKubectlRunner
}

func newClientTestFixture(t *testing.T) *clientTestFixture {
	ret := &clientTestFixture{}
	ret.t = t
	ret.ctx = output.CtxForTest()
	ret.runner = &fakeKubectlRunner{}
	ret.client = K8sClient{EnvUnknown, ret.runner, nil, nil, fakePortForwarder}
	return ret
}

func (c clientTestFixture) setOutput(s string) {
	c.runner.stdout = s
}

func (c clientTestFixture) setError(err error) {
	c.runner.err = err
}

func fakePortForwarder(ctx context.Context, restConfig *rest.Config, core v1.CoreV1Interface, namespace string, podID PodID, localPort int, remotePort int) (closer func(), err error) {
	return nil, nil
}

var _ PortForwarder = fakePortForwarder

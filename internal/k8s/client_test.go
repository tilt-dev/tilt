package k8s

import (
	"context"
	"testing"
)

type fakeKubectlRunner struct {
	stdout string
	stderr string
	err    error
}

func (f fakeKubectlRunner) cli(ctx context.Context, cmd string, entities ...K8sEntity) (stdout string, stderr string, err error) {
	return f.stdout, f.stderr, f.err
}

var _ kubectlRunner = fakeKubectlRunner{}

type clientTestFixture struct {
	t      *testing.T
	client KubectlClient
	runner *fakeKubectlRunner
}

func newClientTestFixture(t *testing.T) *clientTestFixture {
	ret := &clientTestFixture{}
	ret.t = t
	ret.runner = &fakeKubectlRunner{}
	ret.client = KubectlClient{EnvUnknown, ret.runner}
	return ret
}

func (c clientTestFixture) setOutput(s string) {
	c.runner.stdout = s
}

func (c clientTestFixture) setError(err error) {
	c.runner.err = err
}

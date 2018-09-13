package k8s

import (
	"bytes"
	"context"
	"testing"
)

type fakeKubectlRunner struct {
	output *bytes.Buffer
	err    error
}

func (f fakeKubectlRunner) cli(ctx context.Context, cmd string, entities ...K8sEntity) (*bytes.Buffer, error) {
	return f.output, f.err
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
	c.runner.output = bytes.NewBufferString(s)
}

func (c clientTestFixture) setError(err error) {
	c.runner.err = err
}

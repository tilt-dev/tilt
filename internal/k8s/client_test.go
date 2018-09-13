package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"testing"
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

func (f fakeKubectlRunner) execWithStdin(ctx context.Context, args []string, stdin *bytes.Reader) (stdout string, stderr string, err error) {
	b, err := ioutil.ReadAll(stdin)
	if err != nil {
		return "", "", fmt.Errorf("reading stdin: %v", err)
	}
	f.calls = append(f.calls, call{argv: args, stdin: string(b)})
	return f.stdout, f.stderr, f.err
}

func (f fakeKubectlRunner) exec(ctx context.Context, args []string) (stdout string, stderr string, err error) {
	f.calls = append(f.calls, call{argv: args})
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

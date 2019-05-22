package k8s

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/testutils/output"
)

func TestLoggingKubectlRunnerNoStdin(t *testing.T) {
	f := newLoggingKubectlRunnerFixture()

	f.fakeRunner.stdout = "foo"
	f.fakeRunner.stderr = "bar"
	stdout, stderr, err := f.runner.exec(f.ctx, []string{"hello", "goodbye"})
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	assert.Equal(t, []call{{argv: []string{"hello", "goodbye"}}}, f.fakeRunner.calls)

	assert.Equal(t, "foo", stdout)
	assert.Equal(t, "bar", stderr)

	l := f.log()
	assert.Contains(t, l, `Running: ["kubectl" "hello" "goodbye"]`)
	assert.Contains(t, l, `stdout: 'foo'`)
	assert.Contains(t, l, `stderr: 'bar'`)
}

func TestLoggingKubectlRunnerStdin(t *testing.T) {
	f := newLoggingKubectlRunnerFixture()

	input := "some yaml"
	f.fakeRunner.stdout = "foo"
	f.fakeRunner.stderr = "bar"
	stdout, stderr, err := f.runner.execWithStdin(f.ctx, []string{"hello", "goodbye"}, strings.NewReader(input))
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	assert.Equal(t, []call{{
		argv:  []string{"hello", "goodbye"},
		stdin: input,
	}}, f.fakeRunner.calls)

	assert.Equal(t, "foo", stdout)
	assert.Equal(t, "bar", stderr)

	l := f.log()
	assert.Contains(t, l, `Running: ["kubectl" "hello" "goodbye"]`)
	assert.Contains(t, l, `stdout: 'foo'`)
	assert.Contains(t, l, `stderr: 'bar'`)
	assert.Contains(t, l, `stdin: 'some yaml'`)
}

type loggingKubectlRunnerFixture struct {
	runner     loggingKubectlRunner
	fakeRunner *fakeKubectlRunner
	ctx        context.Context
	w          *bytes.Buffer
}

func newLoggingKubectlRunnerFixture() *loggingKubectlRunnerFixture {
	fakeRunner := &fakeKubectlRunner{}
	runner := loggingKubectlRunner{
		logLevel: logger.InfoLvl,
		runner:   fakeRunner,
	}

	w := &bytes.Buffer{}
	ctx := output.ForkedCtxForTest(w)

	return &loggingKubectlRunnerFixture{
		runner,
		fakeRunner,
		ctx,
		w,
	}
}

func (f *loggingKubectlRunnerFixture) log() string {
	return f.w.String()
}

package k8s

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/testutils"
)

func TestLoggingKubectlRunnerNoStdin(t *testing.T) {
	f := newLoggingKubectlRunnerFixture()

	f.fakeRunner.stdout = "foo"
	f.fakeRunner.stderr = "bar"
	f.runner.kubectlLogLevel = 6
	stdout, stderr, err := f.runner.exec(f.ctx, []string{"hello", "goodbye"})
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	assert.Equal(t, []call{{argv: []string{"-v", "6", "hello", "goodbye"}}}, f.fakeRunner.calls)

	assert.Equal(t, "foo", stdout)
	assert.Equal(t, "bar", stderr)

	l := f.log()
	assert.Contains(t, l, `Running: ["kubectl" "-v" "6" "hello" "goodbye"]`)
	assert.Contains(t, l, `stdout: 'foo'`)
	assert.Contains(t, l, `stderr: 'bar'`)
}

func TestLoggingKubectlRunnerStdin(t *testing.T) {
	f := newLoggingKubectlRunnerFixture()

	input := "some yaml"
	f.fakeRunner.stdout = "foo"
	f.fakeRunner.stderr = "bar"
	f.runner.kubectlLogLevel = 6
	stdout, stderr, err := f.runner.execWithStdin(f.ctx, []string{"hello", "goodbye"}, input)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	assert.Equal(t, []call{{
		argv:  []string{"-v", "6", "hello", "goodbye"},
		stdin: input,
	}}, f.fakeRunner.calls)

	assert.Equal(t, "foo", stdout)
	assert.Equal(t, "bar", stderr)

	l := f.log()
	assert.Contains(t, l, `Running: ["kubectl" "-v" "6" "hello" "goodbye"]`)
	assert.Contains(t, l, `stdout: 'foo'`)
	assert.Contains(t, l, `stderr: 'bar'`)
	assert.Contains(t, l, `stdin: 'some yaml'`)
}

func TestLoggingKubectlRunnerStdinLogLevelNone(t *testing.T) {
	f := newLoggingKubectlRunnerFixture()

	input := "some yaml"
	f.fakeRunner.stdout = "foo"
	f.fakeRunner.stderr = "bar"
	f.runner.kubectlLogLevel = 0
	stdout, stderr, err := f.runner.execWithStdin(f.ctx, []string{"hello", "goodbye"}, input)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	assert.Equal(t, []call{{
		// args are unmodified for log level 0
		argv:  []string{"hello", "goodbye"},
		stdin: input,
	}}, f.fakeRunner.calls)

	assert.Equal(t, "foo", stdout)
	assert.Equal(t, "bar", stderr)

	assert.Equal(t, "", f.log())
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
		runner: fakeRunner,
	}

	w := &bytes.Buffer{}
	ctx, _, _ := testutils.ForkedCtxAndAnalyticsForTest(w)

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

package containerupdate

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils"
)

var toDelete = []string{"/foo/delete_me", "/bar/me_too"}
var (
	cmdA = model.Cmd{Argv: []string{"a"}}
	cmdB = model.Cmd{Argv: []string{"b", "bar", "baz"}}
)
var cmds = []model.Cmd{cmdA, cmdB}

func TestUpdateContainerDoesntSupportRestart(t *testing.T) {
	f := newExecFixture(t)

	err := f.ecu.UpdateContainer(f.ctx, TestDeployInfo, newReader("boop"), toDelete, cmds, false)
	if assert.NotNil(t, err, "expect Exec UpdateContainer to fail if !hotReload") {
		assert.Contains(t, err.Error(), "ExecUpdater does not support `restart_container()` step")
	}
}

func TestUpdateContainerDeletesFiles(t *testing.T) {
	f := newExecFixture(t)

	// No files to delete
	err := f.ecu.UpdateContainer(f.ctx, TestDeployInfo, newReader("boop"), nil, cmds, true)
	if err != nil {
		t.Fatal(err)
	}

	for _, call := range f.kCli.ExecCalls {
		if len(call.Cmd) >= 1 && call.Cmd[0] == "rm" {
			t.Errorf("found kubernetes exec `rm` call, expected none b/c no files to delete")
			t.Fail()
		}
	}

	// Two files to delete
	err = f.ecu.UpdateContainer(f.ctx, TestDeployInfo, newReader("boop"), toDelete, cmds, true)
	if err != nil {
		t.Fatal(err)
	}
	var rmCmd []string
	for _, call := range f.kCli.ExecCalls {
		if len(call.Cmd) >= 1 && call.Cmd[0] == "rm" {
			rmCmd = call.Cmd
			break
		}
	}
	if len(rmCmd) == 0 {
		t.Errorf("no `rm` cmd found, expected one b/c we specified files to delete")
		t.Fail()
	}

	expectedRmCmd := []string{"rm", "-rf", "/foo/delete_me", "/bar/me_too"}
	assert.Equal(t, expectedRmCmd, rmCmd)
}

func TestUpdateContainerTarsArchive(t *testing.T) {
	f := newExecFixture(t)

	err := f.ecu.UpdateContainer(f.ctx, TestDeployInfo, newReader("hello world"), nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	expectedCmd := []string{"tar", "-C", "/", "-x", "-v", "-f", "-"}
	if assert.Len(t, f.kCli.ExecCalls, 1, "expect exactly 1 k8s exec call") {
		call := f.kCli.ExecCalls[0]
		assert.Equal(t, expectedCmd, call.Cmd)
		assert.Equal(t, []byte("hello world"), call.Stdin)
	}
}

func TestUpdateContainerRunsCommands(t *testing.T) {
	f := newExecFixture(t)

	err := f.ecu.UpdateContainer(f.ctx, TestDeployInfo, newReader("hello world"), nil, cmds, true)
	if err != nil {
		t.Fatal(err)
	}

	if assert.Len(t, f.kCli.ExecCalls, 3, "expect exactly 3 k8s exec calls") {
		// second and third calls should be our cmd runs
		assert.Equal(t, cmdA.Argv, f.kCli.ExecCalls[1].Cmd)
		assert.Equal(t, cmdB.Argv, f.kCli.ExecCalls[2].Cmd)
	}
}

type execUpdaterFixture struct {
	t    testing.TB
	ctx  context.Context
	kCli *k8s.FakeK8sClient
	ecu  *ExecUpdater
}

func newExecFixture(t testing.TB) *execUpdaterFixture {
	fakeCli := k8s.NewFakeK8sClient()
	cu := &ExecUpdater{
		kCli: fakeCli,
	}
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()

	return &execUpdaterFixture{
		t:    t,
		ctx:  ctx,
		kCli: fakeCli,
		ecu:  cu,
	}
}

func newReader(contents string) io.Reader {
	return bytes.NewBuffer([]byte(contents))
}

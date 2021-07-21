package localexec

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestDefaultEnv(t *testing.T) {
	env := DefaultEnv(8000, "tilt.local")
	env.environ = func() []string { return nil }
	l := logger.NewTestLogger(bytes.NewBuffer(nil))
	cmd := &exec.Cmd{}
	cmdModel := model.Cmd{Argv: []string{"x"}, Env: []string{"x=y"}}
	env.populateExecCmd(cmd, cmdModel, l)
	assert.Equal(t, cmd.Env, []string{
		"LINES=24",
		"COLUMNS=80",
		"PYTHONUNBUFFERED=1",
		"TILT_PORT=8000",
		"TILT_HOST=tilt.local",
		"TILT_DISABLE_ANALYTICS=1",
		"x=y",
	})
}

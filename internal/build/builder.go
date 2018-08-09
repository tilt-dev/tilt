package build

import (
	"context"
	"os/exec"
)

type Builder interface {
	Build(ctx context.Context, pathToFS string, cmds []Cmd) ([]Output, string, error)
}

type Output struct {
	CombinedOutput []byte
	Error          exec.ExitError
}

type Cmd struct {
	Argv []string
}

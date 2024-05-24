package model

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type Cmd struct {
	Argv      []string
	Dir       string
	Env       []string
	EchoOff   bool
	StdinMode v1alpha1.StdinMode
}

func (c Cmd) IsShellStandardForm() bool {
	return len(c.Argv) == 3 && c.Argv[0] == "sh" && c.Argv[1] == "-c" && !strings.Contains(c.Argv[2], "\n")
}

func (c Cmd) IsWindowsStandardForm() bool {
	return len(c.Argv) == 4 && c.Argv[0] == "cmd" && c.Argv[1] == "/S" && c.Argv[2] == "/C"
}

// Get the script when the shell is in standard form.
// Panics if the command is not in shell standard form.
func (c Cmd) ShellStandardScript() string {
	if !c.IsShellStandardForm() {
		panic(fmt.Sprintf("Not in shell standard form: %+v", c))
	}
	return c.Argv[2]
}

func (c Cmd) EntrypointStr() string {
	if c.IsShellStandardForm() {
		return fmt.Sprintf("ENTRYPOINT %s", c.Argv[2])
	}

	quoted := make([]string, len(c.Argv))
	for i, arg := range c.Argv {
		quoted[i] = fmt.Sprintf("%q", arg)
	}
	return fmt.Sprintf("ENTRYPOINT [%s]", strings.Join(quoted, ", "))
}

func (c Cmd) RunStr() string {
	if c.IsShellStandardForm() {
		return fmt.Sprintf("RUN %s", c.Argv[2])
	}

	quoted := make([]string, len(c.Argv))
	for i, arg := range c.Argv {
		quoted[i] = fmt.Sprintf("%q", arg)
	}
	return fmt.Sprintf("RUN [%s]", strings.Join(quoted, ", "))
}

func ArgListToString(args []string) string {
	return Cmd{Argv: args}.String()
}

func (c Cmd) String() string {
	if c.IsShellStandardForm() {
		return c.Argv[2]
	}

	if c.IsWindowsStandardForm() {
		return c.Argv[3]
	}

	quoted := make([]string, len(c.Argv))
	for i, arg := range c.Argv {
		if strings.Contains(arg, " ") {
			quoted[i] = fmt.Sprintf("%q", arg)
		} else {
			quoted[i] = arg
		}
	}
	return strings.Join(quoted, " ")
}

func (c Cmd) Empty() bool {
	return len(c.Argv) == 0
}

// Create a shell command for running on the Host OS
func ToHostCmd(cmd string) Cmd {
	if cmd == "" {
		return Cmd{}
	}
	if runtime.GOOS == "windows" {
		return ToBatCmd(cmd)
	}
	return ToUnixCmd(cmd)
}

func ToHostCmdInDir(cmd string, dir string) Cmd {
	c := ToHostCmd(cmd)
	c.Dir = dir
	return c
}

func ToHostCmdInDirWithEnv(cmd string, dir string, env []string) Cmd {
	c := ToHostCmdInDir(cmd, dir)
	c.Env = env
	return c
}

// 🦇🦇🦇
// Named in honor of Bazel
// https://docs.bazel.build/versions/master/be/general.html#genrule.cmd_bat
func ToBatCmd(cmd string) Cmd {
	if cmd == "" {
		return Cmd{}
	}
	// from https://docs.docker.com/engine/reference/builder/#run
	//
	// NOTE(nick): cmd /S /C does not handle multi-line strings correctly.
	// It will execute the first line, then exit. Should we warn or error about this?
	//
	// The TrimSpace ensures we at least execute the first non-empty line.
	return Cmd{Argv: []string{"cmd", "/S", "/C", strings.TrimSpace(cmd)}}
}

func ToUnixCmd(cmd string) Cmd {
	if cmd == "" {
		return Cmd{}
	}

	// trim spurious spaces and execute them in shell.
	return Cmd{Argv: []string{"sh", "-c", strings.TrimSpace(cmd)}}
}

func ToUnixCmdInDir(cmd string, dir string) Cmd {
	c := ToUnixCmd(cmd)
	c.Dir = dir
	return c
}

func ToUnixCmds(cmds []string) []Cmd {
	res := make([]Cmd, len(cmds))
	for i, cmd := range cmds {
		res[i] = ToUnixCmd(cmd)
	}
	return res
}

func ToRun(cmd Cmd) Run {
	return Run{Cmd: cmd}
}

func ToRuns(cmds []Cmd) []Run {
	res := make([]Run, len(cmds))
	for i, cmd := range cmds {
		res[i] = ToRun(cmd)
	}
	return res
}

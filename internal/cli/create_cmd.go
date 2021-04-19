package cli

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// A human-friendly CLI for creating cmds.
//
// The name is unfortunate.
//
// (as opposed to the machine-friendly CLIs of create -f or apply -f)
type createCmdCmd struct {
	helper *createHelper

	dir         string
	env         []string
	filewatches []string
}

var _ tiltCmd = &createCmdCmd{}

func newCreateCmdCmd() *createCmdCmd {
	helper := newCreateHelper()
	return &createCmdCmd{
		helper: helper,
	}
}

func (c *createCmdCmd) name() model.TiltSubcommand { return "cmd" }

func (c *createCmdCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "cmd NAME COMMAND [ARG...]",
		DisableFlagsInUseLine: true,
		Short:                 "Create a local command in a running tilt session",
		Long: `Create a local command in a running tilt session.

Intended to compose with other Tilt APIs that
can restart the command or monitor its status.

COMMAND should be an executable. A shell script will not work.

To run a shell script, use 'sh -c' (as shown in the examples).
`,
		Args: cobra.MinimumNArgs(2),
		Example: `
tilt create cmd my-cmd echo hello world

tilt create cmd my-cmd sh -c "echo hi && echo bye"
`,
	}

	// Interpret any flags after the object name as part of the
	// command args.
	cmd.Flags().SetInterspersed(false)

	cmd.Flags().StringVarP(&c.dir, "workdir", "w", "",
		"Working directory of the command. If not specified, uses the current working directory.")
	cmd.Flags().StringArrayVarP(&c.env, "env", "e", nil,
		"Set environment variables in the form NAME=VALUE.")
	cmd.Flags().StringSliceVar(&c.filewatches, "filewatch", nil,
		("Re-run the command whenever the named filewatches detect a change. " +
			"See 'tilt create filewatch' for more."))

	c.helper.addFlags(cmd)

	return cmd
}

func (c *createCmdCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	cmdTags := engineanalytics.CmdTags(map[string]string{})
	a.Incr("cmd.create-cmd", cmdTags.AsMap())
	defer a.Flush(time.Second)

	err := c.helper.interpretFlags(ctx)
	if err != nil {
		return err
	}

	cmd, err := c.object(args)
	if err != nil {
		return err
	}

	return c.helper.create(ctx, cmd)
}

// Interprets the flags specified on the commandline to the Cmd to create.
func (c *createCmdCmd) object(args []string) (*v1alpha1.Cmd, error) {
	name := args[0]
	command := args[1:]

	dir, err := c.workdir()
	if err != nil {
		return nil, err
	}

	env := c.env
	restartOn := c.restartOn()
	cmd := v1alpha1.Cmd{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.CmdSpec{
			Args:      command,
			Dir:       dir,
			Env:       env,
			RestartOn: restartOn,
		},
	}
	return &cmd, nil
}

// Determine the working directory of the command.
func (c *createCmdCmd) workdir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	if c.dir == "" {
		return cwd, nil
	}
	if filepath.IsAbs(c.dir) {
		return c.dir, nil
	}
	return filepath.Join(cwd, c.dir), nil
}

// Determine the restart conditions of the command.
func (c *createCmdCmd) restartOn() *v1alpha1.RestartOnSpec {
	return &v1alpha1.RestartOnSpec{
		FileWatches: c.filewatches,
	}
}

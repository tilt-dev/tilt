package cli

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// A human-friendly CLI for creating extensions.
type createExtCmd struct {
	helper *createHelper
	cmd    *cobra.Command

	repoName string
	repoPath string
}

var _ tiltCmd = &createExtCmd{}

func newCreateExtCmd(streams genericclioptions.IOStreams) *createExtCmd {
	helper := newCreateHelper(streams)
	return &createExtCmd{
		helper: helper,
	}
}

func (c *createExtCmd) name() model.TiltSubcommand { return "create" }

func (c *createExtCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "ext NAME [ARG...]",
		DisableFlagsInUseLine: true,
		Short:                 "Register an extension.",
		Long: `Register an extension with a running Tilt instance.

An extension will load a set of services into your dev environment.

These might be services you need to run your app, or servers
that add functionality to Tilt itself.

Assumes that an extension repo has already been registered
with 'tilt create repo' or in the Tiltfile.
`,
		Args: cobra.MinimumNArgs(1),
		Example: `
# Installs the extension from the extension repo 'default' under the path './cancel'.
tilt create ext cancel

# Installs the extension from the extension repo 'default' under
# and with custom argument '--namespaces=default' passed to the extension.
tilt create ext my-kubefwd --path=./kubefwd -- --namespaces=default

# Installs the extension from the extension repo 'dev' under the path './cancel'
tilt create ext cancel --repo=dev
`,
	}

	cmd.Flags().StringVar(&c.repoName, "repo", "default",
		"The name of the extension repo (list existing repos with 'tilt get repo')")
	cmd.Flags().StringVar(&c.repoPath, "path", "",
		"The path of the extension. If not specified, defaults to the extension name.")

	c.helper.addFlags(cmd)
	c.cmd = cmd

	return cmd
}

func (c *createExtCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	cmdTags := engineanalytics.CmdTags(map[string]string{})
	a.Incr("cmd.create-ext", cmdTags.AsMap())
	defer a.Flush(time.Second)

	err := c.helper.interpretFlags(ctx)
	if err != nil {
		return err
	}

	name := args[0]
	extArgs := []string{}
	if c.cmd.ArgsLenAtDash() != -1 {
		extArgs = args[c.cmd.ArgsLenAtDash():]
	}

	return c.helper.create(ctx, c.object(name, extArgs))
}

func (c *createExtCmd) object(name string, extArgs []string) *v1alpha1.Extension {
	repoName := c.repoName
	repoPath := c.repoPath
	if repoPath == "" {
		repoPath = name
	}
	return &v1alpha1.Extension{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.ExtensionSpec{
			RepoName: repoName,
			RepoPath: repoPath,
			Args:     extArgs,
		},
	}
}

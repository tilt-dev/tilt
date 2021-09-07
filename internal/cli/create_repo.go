package cli

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// A human-friendly CLI for creating extension repos.
type createRepoCmd struct {
	helper *createHelper

	ref string
}

var _ tiltCmd = &createRepoCmd{}

func newCreateRepoCmd() *createRepoCmd {
	helper := newCreateHelper()
	return &createRepoCmd{
		helper: helper,
	}
}

func (c *createRepoCmd) name() model.TiltSubcommand { return "create" }

func (c *createRepoCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "repo NAME URL [ARG...]",
		DisableFlagsInUseLine: true,
		Short:                 "Register an extension repository.",
		Long: `Register a repository for loading Tilt extensions.

Tilt supports both git-hosted and local filesystem repositories.
`,
		Args: cobra.MinimumNArgs(2),
		Example: `
tilt create repo default https://github.com/tilt-dev/tilt-extensions
tilt create repo default file:///home/user/src/tilt-extensions
tilt create repo default https://github.com/tilt-dev/tilt-extensions --ref=SHA
`,
	}

	cmd.Flags().StringVar(&c.ref, "ref", "",
		"Git reference to sync the repository to.")

	c.helper.addFlags(cmd)

	return cmd
}

func (c *createRepoCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	cmdTags := engineanalytics.CmdTags(map[string]string{})
	a.Incr("cmd.create-repo", cmdTags.AsMap())
	defer a.Flush(time.Second)

	err := c.helper.interpretFlags(ctx)
	if err != nil {
		return err
	}

	return c.helper.create(ctx, c.object(args))
}

func (c *createRepoCmd) object(args []string) *v1alpha1.ExtensionRepo {
	name := args[0]
	url := args[1]
	return &v1alpha1.ExtensionRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.ExtensionRepoSpec{
			URL: url,
			Ref: c.ref,
		},
	}
}

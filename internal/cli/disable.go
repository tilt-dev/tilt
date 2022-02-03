package cli

import (
	"context"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

type disableCmd struct {
	all bool
}

func newDisableCmd() *disableCmd {
	return &disableCmd{}
}

func (c *disableCmd) name() model.TiltSubcommand { return "disable" }

func (c *disableCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "disable {-all | <resource>...}",
		DisableFlagsInUseLine: true,
		Short:                 "Disables resources",
		Long: `Disables the specified resources in Tilt.

# disables the resources named 'frontend' and 'backend'
tilt disable frontend backend

# disables all resources
tilt disable --all`,
	}

	cmd.Flags().BoolVar(&c.all, "all", false, "Disable all resources")

	addConnectServerFlags(cmd)

	return cmd
}

func (c *disableCmd) run(ctx context.Context, args []string) error {
	ctrlclient, err := newClient(ctx)
	if err != nil {
		return err
	}

	if c.all {
		if len(args) > 0 {
			return errors.New("cannot use --all with resource names")
		}
	} else if len(args) == 0 {
		return errors.New("must specify at least one resource")
	}

	a := analytics.Get(ctx)
	cmdTags := engineanalytics.CmdTags(map[string]string{})
	cmdTags["all"] = strconv.FormatBool(c.all)
	a.Incr("cmd.disable", cmdTags.AsMap())
	defer a.Flush(time.Second)

	names := make(map[string]bool)
	for _, name := range args {
		names[name] = true
	}

	unselectedState := v1alpha1.DisableStatePending
	if c.all {
		unselectedState = v1alpha1.DisableStateDisabled
	}

	err = changeEnabledResources(ctx, ctrlclient, args, v1alpha1.DisableStateDisabled, unselectedState)
	if err != nil {
		return err
	}

	return nil
}

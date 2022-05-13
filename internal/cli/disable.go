package cli

import (
	"context"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/pkg/model"
)

type disableCmd struct {
	all    bool
	labels []string
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

	cmd.Flags().StringSliceVarP(&c.labels, "labels", "l", c.labels, "Disable all resources with the specified labels")
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
	} else if len(args) == 0 && len(c.labels) == 0 {
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

	err = changeEnabledResources(ctx, ctrlclient, args, enableOptions{enable: false, all: c.all, only: false, labels: c.labels})
	if err != nil {
		return err
	}

	return nil
}

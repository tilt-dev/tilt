package cli

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

type enableCmd struct {
	all  bool
	only bool
}

func newEnableCmd() *enableCmd {
	return &enableCmd{}
}

func (c *enableCmd) name() model.TiltSubcommand { return "enable" }

func (c *enableCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "enable {-all | [--only] <resource>...}",
		DisableFlagsInUseLine: true,
		Short:                 "Enables resources",
		Long: `Enables the specified resources in Tilt.

# enables the resources named 'frontend' and 'backend'
tilt enable frontend backend

# enables frontend and backend and disables all others
tilt enable --only frontend backend

# enables all resources
tilt enable --all
`,
	}

	addConnectServerFlags(cmd)
	cmd.Flags().BoolVar(&c.only, "only", false, "Enable the specified resources, disable all others")
	cmd.Flags().BoolVar(&c.all, "all", false, "Enable all resources")

	return cmd
}

func (c *enableCmd) run(ctx context.Context, args []string) error {
	ctrlclient, err := newClient(ctx)
	if err != nil {
		return err
	}

	if c.all {
		if c.only {
			return errors.New("cannot use --all with --only")
		} else if len(args) > 0 {
			return errors.New("cannot use --all with resource names")
		}
	} else if len(args) == 0 {
		return errors.New("must specify at least one resource")
	}

	a := analytics.Get(ctx)
	cmdTags := engineanalytics.CmdTags(map[string]string{})
	cmdTags["only"] = strconv.FormatBool(c.only)
	cmdTags["all"] = strconv.FormatBool(c.all)
	a.Incr("cmd.enable", cmdTags.AsMap())
	defer a.Flush(time.Second)

	names := make(map[string]bool)
	for _, name := range args {
		names[name] = true
	}

	unselectedState := v1alpha1.DisableStatePending
	if c.only {
		unselectedState = v1alpha1.DisableStateDisabled
	} else if c.all {
		unselectedState = v1alpha1.DisableStateEnabled
	}
	err = changeEnabledResources(ctx, ctrlclient, args, v1alpha1.DisableStateEnabled, unselectedState)
	if err != nil {
		return err
	}

	return nil
}

// Changes which resources are enabled in Tilt.
// For resources in `selectedResources`, ensures their disabled state is `selectedState`.
// For all other resources:
// - if `unselectedState` is Enabled or Disabled, ensure their disabled state is that
// - otherwise, ignore them
func changeEnabledResources(
	ctx context.Context,
	cli client.Client,
	selectedResources []string,
	selectedState v1alpha1.DisableState,
	unselectedState v1alpha1.DisableState,
) error {
	var uirs v1alpha1.UIResourceList
	err := cli.List(ctx, &uirs)
	if err != nil {
		return err
	}

	// before making any changes, validate that all selected names actually exist
	uirByName := make(map[string]v1alpha1.UIResource)
	for _, uir := range uirs.Items {
		uirByName[uir.Name] = uir
	}
	selectedResourcesByName := make(map[string]bool)
	for _, name := range selectedResources {
		uir, ok := uirByName[name]
		if !ok {
			return fmt.Errorf("no such resource %q", name)
		}
		if len(uir.Status.DisableStatus.Sources) == 0 {
			return fmt.Errorf("%s cannot be enabled or disabled", name)
		}
		selectedResourcesByName[name] = true
	}

	for _, uir := range uirs.Items {
		// resources w/o disable sources are always enabled (e.g., (Tiltfile))
		if len(uir.Status.DisableStatus.Sources) == 0 {
			continue
		}

		newState := unselectedState
		if selectedResourcesByName[uir.Name] {
			newState = selectedState
		}

		var newIsDisabled bool
		switch newState {
		case v1alpha1.DisableStateEnabled:
			newIsDisabled = false
		case v1alpha1.DisableStateDisabled:
			newIsDisabled = true
		default:
			continue
		}

		for _, source := range uir.Status.DisableStatus.Sources {
			if source.ConfigMap == nil {
				return fmt.Errorf("internal error: resource %s's DisableSource does not have a ConfigMap'", uir.Name)
			}
			cm := &v1alpha1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: source.ConfigMap.Name}}
			_, err := controllerutil.CreateOrUpdate(ctx, cli, cm, func() error {
				if cm.Data == nil {
					cm.Data = make(map[string]string)
				}
				cm.Data[source.ConfigMap.Key] = strconv.FormatBool(newIsDisabled)
				return nil
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

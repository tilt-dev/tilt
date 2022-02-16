/*
Adapted from
https://github.com/kubernetes/kubectl/tree/master/pkg/cmd/wait
*/

/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cli

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/cmd/wait"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/pkg/model"
)

var (
	waitLong = templates.LongDesc(i18n.T(`
		Experimental: Wait for a specific condition on one or many resources.

		The command takes multiple resources and waits until the specified condition
		is seen in the Status field of every given resource.

		A successful message will be printed to stdout indicating when the specified
    condition has been met. You can use -o option to change to output destination.`))

	waitExample = templates.Examples(i18n.T(`
		# Wait for the tiltfile to load.
		tilt wait --for=condition=Ready "uiresource/(Tiltfile)"

		# When used with a Kubernetes resource, waits for the pod
    # to deploy, start running, and pass all readiness probes.
		tilt wait --for=condition=Ready "uiresource/my-kubernetes-deployment"`))
)

type waitCmd struct {
	flags *wait.WaitFlags
}

var _ tiltCmd = &waitCmd{}

func newWaitCmd(streams genericclioptions.IOStreams) *waitCmd {
	flags := wait.NewWaitFlags(nil, streams)
	return &waitCmd{
		flags: flags,
	}
}

func (c *waitCmd) name() model.TiltSubcommand { return "wait" }

func (c *waitCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "wait ([-f FILENAME] | resource.group/resource.name | resource.group [(-l label | --all)]) [--for=delete|--for condition=available]",
		Short:   "Experimental: Wait for a specific condition on one or many resources",
		Long:    waitLong,
		Example: waitExample,

		DisableFlagsInUseLine: true,
	}

	c.flags.AddFlags(cmd)
	addConnectServerFlags(cmd)

	return cmd
}

func (c *waitCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	cmdTags := engineanalytics.CmdTags(map[string]string{})
	a.Incr("cmd.wait", cmdTags.AsMap())
	defer a.Flush(time.Second)

	getter, err := wireClientGetter(ctx)
	if err != nil {
		return err
	}

	c.flags.RESTClientGetter = getter

	o, err := c.flags.ToOptions(args)
	cmdutil.CheckErr(err)
	cmdutil.CheckErr(o.RunWait())

	return nil
}

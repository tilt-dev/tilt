/*
Adapted from
https://github.com/kubernetes/kubectl/tree/master/pkg/cmd/get
*/

/*
Copyright 2014 The Kubernetes Authors.

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
	"os"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/pkg/model"
)

type getCmd struct {
	options *get.GetOptions
	cmd     *cobra.Command
}

var _ tiltCmd = &getCmd{}

func newGetCmd() *getCmd {
	streams := genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr, In: os.Stdin}
	o := get.NewGetOptions("tilt", streams)
	return &getCmd{
		options: o,
	}
}

func (c *getCmd) name() model.TiltSubcommand { return "get" }

func (c *getCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "get TYPE [NAME | -l label]",
		DisableFlagsInUseLine: true,
		Short:                 "Display one or many resources",
	}
	c.cmd = cmd
	o := c.options

	o.PrintFlags.AddFlags(cmd)

	cmd.Flags().BoolVarP(&o.Watch, "watch", "w", o.Watch, "After listing/getting the requested object, watch for changes. Uninitialized objects are excluded if no object name is provided.")
	cmd.Flags().BoolVar(&o.WatchOnly, "watch-only", o.WatchOnly, "Watch for changes to the requested object(s), without listing/getting first.")
	cmd.Flags().BoolVar(&o.IgnoreNotFound, "ignore-not-found", o.IgnoreNotFound, "If the requested object does not exist the command will return exit code 0.")
	cmd.Flags().StringVarP(&o.LabelSelector, "selector", "l", o.LabelSelector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	cmd.Flags().StringVar(&o.FieldSelector, "field-selector", o.FieldSelector, "Selector (field query) to filter on, supports '=', '==', and '!='.(e.g. --field-selector key1=value1,key2=value2). The server only supports a limited number of field queries per type.")
	addConnectServerFlags(cmd)
	return cmd
}

func (c *getCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	cmdTags := engineanalytics.CmdTags(map[string]string{})
	a.Incr("cmd.get", cmdTags.AsMap())
	defer a.Flush(time.Second)

	o := c.options
	getter, err := wireClientGetter(ctx)
	if err != nil {
		return err
	}

	f := cmdutil.NewFactory(getter)
	cmd := c.cmd
	cmdutil.CheckErr(o.Complete(f, cmd, args))
	cmdutil.CheckErr(o.Validate(cmd))
	cmdutil.CheckErr(o.Run(f, cmd, args))
	return nil
}

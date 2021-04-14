/*
Adapted from
https://github.com/kubernetes/kubectl/tree/master/pkg/cmd/apiresources
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
	"k8s.io/kubectl/pkg/cmd/apiresources"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/pkg/model"
)

type apiresourcesCmd struct {
	options *apiresources.APIResourceOptions
	cmd     *cobra.Command
}

var _ tiltCmd = &apiresourcesCmd{}

func newApiresourcesCmd() *apiresourcesCmd {
	streams := genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr, In: os.Stdin}
	o := apiresources.NewAPIResourceOptions(streams)
	return &apiresourcesCmd{
		options: o,
	}
}

func (c *apiresourcesCmd) name() model.TiltSubcommand { return "apiresources" }

func (c *apiresourcesCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api-resources",
		Short: "Print the supported API resources",
	}
	c.cmd = cmd
	o := c.options

	cmd.Flags().BoolVar(&o.NoHeaders, "no-headers", o.NoHeaders, "When using the default or custom-column output format, don't print headers (default print headers).")
	cmd.Flags().StringVarP(&o.Output, "output", "o", o.Output, "Output format. One of: wide|name.")

	cmd.Flags().StringVar(&o.APIGroup, "api-group", o.APIGroup, "Limit to resources in the specified API group.")
	cmd.Flags().BoolVar(&o.Namespaced, "namespaced", o.Namespaced, "If false, non-namespaced resources will be returned, otherwise returning namespaced resources by default.")
	cmd.Flags().StringSliceVar(&o.Verbs, "verbs", o.Verbs, "Limit to resources that support the specified verbs.")
	cmd.Flags().StringVar(&o.SortBy, "sort-by", o.SortBy, "If non-empty, sort list of resources using specified field. The field can be either 'name' or 'kind'.")
	cmd.Flags().BoolVar(&o.Cached, "cached", o.Cached, "Use the cached list of resources if available.")

	addConnectServerFlags(cmd)
	return cmd
}

func (c *apiresourcesCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	cmdTags := engineanalytics.CmdTags(map[string]string{})
	a.Incr("cmd.api-resources", cmdTags.AsMap())
	defer a.Flush(time.Second)

	o := c.options
	getter, err := wireClientGetter(ctx)
	if err != nil {
		return err
	}

	f := cmdutil.NewFactory(getter)
	cmd := c.cmd
	cmdutil.CheckErr(o.Complete(cmd, args))
	cmdutil.CheckErr(o.Validate())
	cmdutil.CheckErr(o.RunAPIResources(cmd, f))
	return nil
}

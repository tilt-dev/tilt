/*
Adapted from
https://github.com/kubernetes/kubectl/tree/master/pkg/cmd/describe
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
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/kubectl/pkg/cmd/describe"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	pkgdescribe "k8s.io/kubectl/pkg/describe"

	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/pkg/model"
)

type describeCmd struct {
	options *describe.DescribeOptions
	cmd     *cobra.Command
}

var _ tiltCmd = &describeCmd{}

func newDescribeCmd(streams genericclioptions.IOStreams) *describeCmd {
	o := &describe.DescribeOptions{
		FilenameOptions: &resource.FilenameOptions{},
		DescriberSettings: &pkgdescribe.DescriberSettings{
			ShowEvents: false,
		},

		CmdParent: "tilt",
		IOStreams: streams,
	}
	return &describeCmd{
		options: o,
	}
}

func (c *describeCmd) name() model.TiltSubcommand { return "describe" }

func (c *describeCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "describe (-f FILENAME | TYPE [NAME_PREFIX | -l label] | TYPE/NAME)",
		DisableFlagsInUseLine: true,
		Short:                 "Show details of a specific resource or group of resources",
	}
	c.cmd = cmd
	o := c.options

	cmdutil.AddFilenameOptionFlags(cmd, o.FilenameOptions, "containing the resources to describe")
	cmd.Flags().StringVarP(&o.Selector, "selector", "l", o.Selector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	addConnectServerFlags(cmd)
	return cmd
}

func (c *describeCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	cmdTags := engineanalytics.CmdTags(map[string]string{})
	a.Incr("cmd.describe", cmdTags.AsMap())
	defer a.Flush(time.Second)

	o := c.options
	getter, err := wireClientGetter(ctx)
	if err != nil {
		return err
	}

	f := cmdutil.NewFactory(getter)
	o.NewBuilder = f.NewBuilder
	o.BuilderArgs = args
	o.Describer = func(mapping *meta.RESTMapping) (pkgdescribe.ResourceDescriber, error) {
		return pkgdescribe.DescriberFn(f, mapping)
	}
	cmdutil.CheckErr(o.Validate())
	cmdutil.CheckErr(o.Run())
	return nil
}

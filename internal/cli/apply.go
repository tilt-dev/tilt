/*
Adapted from
https://github.com/kubernetes/kubectl/tree/master/pkg/cmd/apply
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
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/cmd/apply"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/pkg/model"
)

type applyCmd struct {
	streams genericclioptions.IOStreams
	flags   *apply.ApplyFlags
	cmd     *cobra.Command
}

var _ tiltCmd = &applyCmd{}

func newApplyCmd(streams genericclioptions.IOStreams) *applyCmd {
	return &applyCmd{
		streams: streams,
	}
}

func (c *applyCmd) name() model.TiltSubcommand { return "apply" }

func (c *applyCmd) register() *cobra.Command {
	flags := apply.NewApplyFlags(nil, c.streams)

	cmd := &cobra.Command{
		Use:                   "apply (-f FILENAME | -k DIRECTORY)",
		DisableFlagsInUseLine: true,
		Short:                 "Apply a configuration to a resource by filename or stdin",
	}

	flags.AddFlags(cmd)

	addConnectServerFlags(cmd)

	c.cmd = cmd
	c.flags = flags
	return cmd
}

func (c *applyCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	cmdTags := engineanalytics.CmdTags(map[string]string{})
	a.Incr("cmd.apply", cmdTags.AsMap())
	defer a.Flush(time.Second)

	getter, err := wireClientGetter(ctx)
	if err != nil {
		return err
	}

	f := cmdutil.NewFactory(getter)
	c.flags.Factory = f

	cmd := c.cmd
	o, err := c.flags.ToOptions(cmd, "tilt", args)
	if err != nil {
		return err
	}

	cmdutil.CheckErr(err)
	cmdutil.CheckErr(o.Validate())
	cmdutil.CheckErr(o.Run())
	return nil
}

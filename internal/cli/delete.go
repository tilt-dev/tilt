/*
Adapted from
https://github.com/kubernetes/kubectl/tree/master/pkg/cmd/delete
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
	"k8s.io/kubectl/pkg/cmd/delete"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/pkg/model"
)

type deleteCmd struct {
	streams     genericclioptions.IOStreams
	deleteFlags *delete.DeleteFlags
	cmd         *cobra.Command
}

var _ tiltCmd = &deleteCmd{}

func newDeleteCmd() *deleteCmd {
	streams := genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr, In: os.Stdin}
	deleteFlags := delete.NewDeleteCommandFlags("containing the resource to delete.")
	return &deleteCmd{
		streams:     streams,
		deleteFlags: deleteFlags,
	}
}

func (c *deleteCmd) name() model.TiltSubcommand { return "delete" }

func (c *deleteCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "delete ([-f FILENAME] | [-k DIRECTORY] | TYPE [(NAME | -l label | --all)])",
		DisableFlagsInUseLine: true,
		Short:                 "Delete resources by filenames, stdin, resources and names, or by resources and label selector",
	}
	c.cmd = cmd
	c.deleteFlags.AddFlags(cmd)

	cmdutil.AddDryRunFlag(cmd)

	addConnectServerFlags(cmd)

	return cmd
}

func (c *deleteCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	cmdTags := engineanalytics.CmdTags(map[string]string{})
	a.Incr("cmd.delete", cmdTags.AsMap())
	defer a.Flush(time.Second)

	o, err := c.deleteFlags.ToOptions(nil, c.streams)
	if err != nil {
		return err
	}

	getter, err := wireClientGetter(ctx)
	if err != nil {
		return err
	}

	f := cmdutil.NewFactory(getter)
	cmdutil.CheckErr(err)
	cmdutil.CheckErr(o.Complete(f, args, c.cmd))
	cmdutil.CheckErr(o.Validate())
	cmdutil.CheckErr(o.RunDelete(f))

	return nil
}

/*
Adapted from
https://github.com/kubernetes/kubectl/tree/master/pkg/cmd/create
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
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/cmd/create"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/pkg/model"
)

type createCmd struct {
	streams genericclioptions.IOStreams
	options *create.CreateOptions
	cmd     *cobra.Command
}

var _ tiltCmd = &createCmd{}

func newCreateCmd() *createCmd {
	streams := genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr, In: os.Stdin}
	return &createCmd{
		streams: streams,
	}
}

func (c *createCmd) name() model.TiltSubcommand { return "create" }

func (c *createCmd) register() *cobra.Command {
	o := create.NewCreateOptions(c.streams)

	cmd := &cobra.Command{
		Use:                   "create -f FILENAME",
		DisableFlagsInUseLine: true,
		Short:                 "Create a resource from a file or from stdin.",
	}

	// bind flag structs
	o.RecordFlags.AddFlags(cmd)

	usage := "to use to create the resource"
	cmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, usage)
	cmdutil.AddValidateFlags(cmd)
	cmd.Flags().BoolVar(&o.EditBeforeCreate, "edit", o.EditBeforeCreate, "Edit the API resource before creating")
	cmd.Flags().Bool("windows-line-endings", runtime.GOOS == "windows",
		"Only relevant if --edit=true. Defaults to the line ending native to your platform.")
	cmdutil.AddApplyAnnotationFlags(cmd)
	cmdutil.AddDryRunFlag(cmd)
	cmd.Flags().StringVarP(&o.Selector, "selector", "l", o.Selector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")

	o.PrintFlags.AddFlags(cmd)
	addConnectServerFlags(cmd)

	c.options = o
	c.cmd = cmd

	addCommand(cmd, newCreateFileWatchCmd())
	addCommand(cmd, newCreateCmdCmd())

	return cmd
}

func (c *createCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	cmdTags := engineanalytics.CmdTags(map[string]string{})
	a.Incr("cmd.create", cmdTags.AsMap())
	defer a.Flush(time.Second)

	o := c.options
	getter, err := wireClientGetter(ctx)
	if err != nil {
		return err
	}

	f := cmdutil.NewFactory(getter)
	cmd := c.cmd

	if cmdutil.IsFilenameSliceEmpty(o.FilenameOptions.Filenames, o.FilenameOptions.Kustomize) {
		_, _ = c.streams.ErrOut.Write([]byte("Error: must specify one of -f and -k\n\n"))
		return nil
	}
	cmdutil.CheckErr(o.Complete(f, cmd))
	cmdutil.CheckErr(o.ValidateArgs(cmd, args))
	cmdutil.CheckErr(o.RunCreate(f, cmd))
	return nil
}

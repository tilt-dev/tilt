/*
Adapted from
https://github.com/kubernetes/kubectl/tree/master/pkg/cmd/edit
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
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/cmd/util/editor"

	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/pkg/model"
)

type editCmd struct {
	streams genericclioptions.IOStreams
	options *editor.EditOptions
	cmd     *cobra.Command
}

var _ tiltCmd = &editCmd{}

func newEditCmd() *editCmd {
	streams := genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr, In: os.Stdin}
	return &editCmd{
		streams: streams,
	}
}

func (c *editCmd) name() model.TiltSubcommand { return "edit" }

func (c *editCmd) register() *cobra.Command {
	o := editor.NewEditOptions(editor.NormalEditMode, c.streams)
	o.ValidateOptions = cmdutil.ValidateOptions{EnableValidation: true}

	cmd := &cobra.Command{
		Use:                   "edit (RESOURCE/NAME | -f FILENAME)",
		DisableFlagsInUseLine: true,
		Short:                 "Edit a resource on the server",
	}

	// bind flag structs
	o.RecordFlags.AddFlags(cmd)
	o.PrintFlags.AddFlags(cmd)

	usage := "to use to edit the resource"
	cmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, usage)
	cmdutil.AddValidateOptionFlags(cmd, &o.ValidateOptions)
	cmd.Flags().BoolVarP(&o.OutputPatch, "output-patch", "", o.OutputPatch, "Output the patch if the resource is edited.")
	cmd.Flags().BoolVar(&o.WindowsLineEndings, "windows-line-endings", o.WindowsLineEndings,
		"Defaults to the line ending native to your platform.")
	cmdutil.AddFieldManagerFlagVar(cmd, &o.FieldManager, "kubectl-edit")
	cmdutil.AddApplyAnnotationVarFlags(cmd, &o.ApplyAnnotation)

	addConnectServerFlags(cmd)

	c.options = o
	c.cmd = cmd
	return cmd
}

func (c *editCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	cmdTags := engineanalytics.CmdTags(map[string]string{})
	a.Incr("cmd.edit", cmdTags.AsMap())
	defer a.Flush(time.Second)

	o := c.options
	getter, err := wireClientGetter(ctx)
	if err != nil {
		return err
	}

	f := cmdutil.NewFactory(getter)
	cmdutil.CheckErr(o.Complete(f, args, c.cmd))
	cmdutil.CheckErr(o.Run())
	return nil
}

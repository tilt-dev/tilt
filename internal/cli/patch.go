/*
Adapted from
https://github.com/kubernetes/kubectl/tree/master/pkg/cmd/patch
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
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/cmd/patch"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/pkg/model"
)

var patchTypes = map[string]types.PatchType{"json": types.JSONPatchType, "merge": types.MergePatchType, "strategic": types.StrategicMergePatchType}

var (
	patchLong = `
		Update fields of a resource using strategic merge patch, a JSON merge patch, or a JSON patch.
		JSON and YAML formats are accepted.

    Uses the same semantics as 'kubectl patch':
    https://kubernetes.io/docs/reference/kubectl/cheatsheet/#patching-resources
`
)

type patchCmd struct {
	streams genericclioptions.IOStreams
	options *patch.PatchOptions
	cmd     *cobra.Command
}

var _ tiltCmd = &patchCmd{}

func newPatchCmd() *patchCmd {
	streams := genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr, In: os.Stdin}
	return &patchCmd{
		streams: streams,
	}
}

func (c *patchCmd) name() model.TiltSubcommand { return "patch" }

func (c *patchCmd) register() *cobra.Command {
	o := patch.NewPatchOptions(c.streams)

	cmd := &cobra.Command{
		Use:                   "patch (-f FILENAME | TYPE NAME) [-p PATCH|--patch-file FILE]",
		DisableFlagsInUseLine: true,
		Short:                 "Update fields of a resource",
		Long:                  patchLong,
	}

	o.RecordFlags.AddFlags(cmd)
	o.PrintFlags.AddFlags(cmd)

	cmd.Flags().StringVarP(&o.Patch, "patch", "p", "", "The patch to be applied to the resource JSON file.")
	cmd.Flags().StringVar(&o.PatchFile, "patch-file", "", "A file containing a patch to be applied to the resource.")
	cmd.Flags().StringVar(&o.PatchType, "type", "strategic", fmt.Sprintf("The type of patch being provided; one of %v", sets.StringKeySet(patchTypes).List()))
	cmdutil.AddDryRunFlag(cmd)
	cmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, "identifying the resource to update")
	cmd.Flags().BoolVar(&o.Local, "local", o.Local, "If true, patch will operate on the content of the file, not the server-side resource.")
	addConnectServerFlags(cmd)

	c.cmd = cmd
	c.options = o
	return cmd
}

func (c *patchCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	cmdTags := engineanalytics.CmdTags(map[string]string{})
	a.Incr("cmd.patch", cmdTags.AsMap())
	defer a.Flush(time.Second)

	getter, err := wireClientGetter(ctx)
	if err != nil {
		return err
	}

	f := cmdutil.NewFactory(getter)
	cmd := c.cmd
	o := c.options

	cmdutil.CheckErr(o.Complete(f, cmd, args))
	cmdutil.CheckErr(o.Validate())
	cmdutil.CheckErr(o.RunPatch())
	return nil
}

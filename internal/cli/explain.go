/*
Adapted from
https://github.com/kubernetes/kubectl/tree/master/pkg/cmd/explain
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
	"k8s.io/kubectl/pkg/cmd/explain"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/pkg/model"
)

var (
	explainLong = templates.LongDesc(i18n.T(`
		List the fields for supported resources.
		This command describes the fields associated with each supported API resource.
		Fields are identified via a simple JSONPath identifier:
			<type>.<fieldName>[.<fieldName>]
		Add the --recursive flag to display all of the fields at once without descriptions.
		Information about each field is retrieved from the server in OpenAPI format.`))

	explainExamples = templates.Examples(i18n.T(`
		# Get the documentation of the resource and its fields
		tilt explain cmds
		# Get the documentation of a specific field of a resource
		tilt explain cmds.spec.args`))
)

type explainCmd struct {
	flags *explain.ExplainFlags
}

var _ tiltCmd = &explainCmd{}

func newExplainCmd(streams genericclioptions.IOStreams) *explainCmd {
	f := explain.NewExplainFlags(streams)
	return &explainCmd{
		flags: f,
	}
}

func (c *explainCmd) name() model.TiltSubcommand { return "explain" }

func (c *explainCmd) register() *cobra.Command {

	cmd := &cobra.Command{
		Use:                   "explain RESOURCE",
		DisableFlagsInUseLine: true,
		Short:                 i18n.T("Get documentation for a resource"),
		Long:                  explainLong,
		Example:               explainExamples,
	}
	cmd.Flags().BoolVar(&c.flags.Recursive, "recursive", c.flags.Recursive, "Print the fields of fields (Currently only 1 level deep)")
	cmd.Flags().StringVar(&c.flags.APIVersion, "api-version", c.flags.APIVersion, "Get different explanations for particular API version (API group/version)")

	// TODO(nick): Currently, tilt explain must connect to a running tilt
	// environment.  But there's not really a fundamental reason why we couldn't
	// fall back to a headless server, like 'tilt dump openapi' does.
	addConnectServerFlags(cmd)

	return cmd
}

func (c *explainCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	cmdTags := engineanalytics.CmdTags(map[string]string{})
	a.Incr("cmd.explain", cmdTags.AsMap())
	defer a.Flush(time.Second)

	getter, err := wireClientGetter(ctx)
	if err != nil {
		return err
	}

	f := cmdutil.NewFactory(getter)
	o, err := c.flags.ToOptions(f, "tilt", args)
	cmdutil.CheckErr(err)
	cmdutil.CheckErr(o.Validate())
	cmdutil.CheckErr(o.Run())
	return nil
}

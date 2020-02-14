package cli

import (
	goflag "flag"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/kubectl/pkg/cmd/apply"
	"k8s.io/kubectl/pkg/cmd/delete"
	"k8s.io/kubectl/pkg/cmd/replace"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/logs"
)

// Currently, the implementation of `kubectl apply` is a complex protocol
// between kubectl and the API server.
//
// The long-term plan to fix this is to move all of the logic to the server side.
// For discussion on this, see:
// https://github.com/kubernetes/enhancements/issues/555
// https://github.com/kubernetes/enhancements/blob/master/keps/sig-api-machinery/0006-apply.md
//
// In the meantime, there is no simple client-go implementation of the `apply`
// protocol. For a long time, Tilt shelled out to the user's kubectl.
//
// Shelling out to kubectl has some downsides:
// 1) We use two different versions of client-go: Tilt's and kubectl's
// 2) The user's kubectl may not have features that Tilt needs
// 3) Rely on the user to have installed kubectl directly.
//
// The `tilt kubectl` command is currently an experiment.
// Rather than re-implement kubectl apply in client-go, we link
// in the kubectl commands we need and shell out.
func newKubectlCmd() *cobra.Command {
	result := &cobra.Command{
		Use:    "kubectl",
		Short:  "kubectl controls the Kubernetes cluster manager",
		Hidden: true,
	}

	genericFlags := genericclioptions.NewConfigFlags(true)
	genericFlags.AddFlags(result.PersistentFlags())
	matchVersionFlags := cmdutil.NewMatchVersionFlags(genericFlags)
	matchVersionFlags.AddFlags(result.PersistentFlags())

	f := cmdutil.NewFactory(matchVersionFlags)
	ioStreams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	cmdApply := apply.NewCmdApply("kubectl", f, ioStreams)

	// TODO(nick): It might make more sense to implement replace and delete with client-go.
	cmdReplace := replace.NewCmdReplace(f, ioStreams)
	cmdDelete := delete.NewCmdDelete(f, ioStreams)

	result.AddCommand(cmdApply)
	result.AddCommand(cmdReplace)
	result.AddCommand(cmdDelete)
	return result
}

// Flag initialization that only happens if we're acting as a kubernetes client.
// Adapted from
// https://github.com/kubernetes/kubernetes/tree/master/cmd/kubectl
//
// Returns a flush() function to be passed to a defer.
func preKubectlCmdInit() func() {
	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	logs.InitLogs()
	return func() {
		logs.FlushLogs()
	}
}

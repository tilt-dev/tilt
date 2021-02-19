package cli

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/spf13/cobra"

	"github.com/tilt-dev/tilt/pkg/model"
)

type kubeconfigPathCmd struct {
}

var _ tiltCmd = &kubeconfigPathCmd{}

func newKubeconfigPathCmd() *kubeconfigPathCmd {
	return &kubeconfigPathCmd{}
}

func (c *kubeconfigPathCmd) name() model.TiltSubcommand { return "kubeconfig-path" }

func (c *kubeconfigPathCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "kubeconfig-path",
		Short:   "Prints out a path to a KUBECONFIG for querying the Tilt apiserver",
		Example: "KUBECONFIG=$(tilt alpha kubeconfig-path) kubectl api-resources",
	}

	addConnectServerFlags(cmd)

	return cmd
}

func (c *kubeconfigPathCmd) run(ctx context.Context, args []string) error {
	tmpfile, err := ioutil.TempFile("", "tilt-kubeconfig")
	if err != nil {
		return err
	}
	defer tmpfile.Close()

	// Don't worry the password isn't real.
	// The tilt-apiserver binds on localhost only by default with no auth.
	_, err = fmt.Fprintf(tmpfile, `apiVersion: v1
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: http://%s:%d
  name: tilt-apiserver
contexts:
- context:
    cluster: tilt-apiserver
    user: tilt-apiserver
  name: tilt-apiserver
current-context: tilt-apiserver
kind: Config
preferences: {}
users:
- name: tilt-apiserver
  user:
    username: corgi
    password: charge!!!
`, provideWebHost(), provideWebPort())

	if err != nil {
		return err
	}

	fmt.Printf("%s", tmpfile.Name())

	return nil
}

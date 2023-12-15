package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/core/kubernetesdiscovery"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type shellCmd struct {
	streams   genericclioptions.IOStreams
	container string
	execer    ShellExecer
}

func newShellCmd(streams genericclioptions.IOStreams) *shellCmd {
	return &shellCmd{
		streams: streams,
		execer:  realShellExecer{},
	}
}

func (c *shellCmd) name() model.TiltSubcommand { return "shell" }

func (c *shellCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "shell [<resource-name>]",
		DisableFlagsInUseLine: true,
		Short:                 "Opens a shell into a container running in Tilt",
		Long: `Opens a shell into a container running in Tilt.

Given a resource name, finds the Kubernetes Pod in that resource,
and opens an interactive shell.

By default, in order, we'll try:
- kubectl shell
- kubectl exec -it <pod> -- bash
- kubectl exec -it <pod> -- sh

Currently only works on MacOS and Linux.`,
		Args: cobra.ExactArgs(1),
	}

	addConnectServerFlags(cmd)
	cmd.Flags().StringVarP(&c.container, "container", "c", "",
		"Name of the container within the pod. Only required if there is more than 1 container.")

	return cmd
}

func (c *shellCmd) run(ctx context.Context, args []string) error {
	ctx = logger.WithLogger(ctx, logger.NewLogger(logger.Get(ctx).Level(), c.streams.ErrOut))

	ctrlclient, err := newClient(ctx)
	if err != nil {
		return err
	}

	resourceName := args[0]
	var uiResource v1alpha1.UIResource
	err = ctrlclient.Get(ctx, types.NamespacedName{Name: resourceName}, &uiResource)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("resource %s not found. To see available resources, run:\ntilt get uiresources", resourceName)
		}
		return fmt.Errorf("looking up resource %s: %v", resourceName, err)
	}

	// TODO(nicks): Add Docker Compose support.
	if uiResource.Status.K8sResourceInfo == nil {
		return fmt.Errorf("resource %s is not a Kubernetes resource. Only Kubernetes pods currently supported",
			resourceName)
	}

	return c.runK8s(ctx, ctrlclient, resourceName)
}

func (c *shellCmd) runK8s(ctx context.Context, ctrlclient client.Client, resourceName string) error {
	// TODO(nicks): Wait for the pod to become ready?
	var k8sDisco v1alpha1.KubernetesDiscovery
	err := ctrlclient.Get(ctx, types.NamespacedName{Name: resourceName}, &k8sDisco)
	if err != nil {
		return fmt.Errorf("looking up kubernetes status %s: %v", resourceName, err)
	}

	pod := kubernetesdiscovery.PickBestPortForwardPod(&k8sDisco)
	if pod == nil {
		return fmt.Errorf("no pod found for resource %s", resourceName)
	}

	co, err := c.selectContainer(pod)
	if err != nil {
		return err
	}

	kubectlBinary, err := c.execer.LookPath("kubectl")
	if err != nil || kubectlBinary == "" {
		return fmt.Errorf("could not find kubectl: %v", err)
	}

	// Fetch the kubeconfig that tilt manages, rather than using the kubeconfig of the current shell.
	var cluster v1alpha1.Cluster
	err = ctrlclient.Get(ctx, types.NamespacedName{Name: "default"}, &cluster)
	if err != nil {
		return fmt.Errorf("looking up cluster: %v", err)
	}
	if cluster.Status.Connection == nil || cluster.Status.Connection.Kubernetes == nil {
		return fmt.Errorf("kubernetes cluster not connected: %v", cluster.Status.Error)
	}

	env := append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", cluster.Status.Connection.Kubernetes.ConfigPath))

	hasKubectlShell, err := c.execer.LookPath("kubectl-shell")
	if err == nil && hasKubectlShell != "" {
		cmd := model.Cmd{Argv: []string{"kubectl", "shell", pod.Name, "-n", pod.Namespace, "-c", co.Name}}
		logger.Get(ctx).Infof("Running: %v", cmd)
		return c.execer.Exec(kubectlBinary, cmd.Argv, env)
	}

	k8sClient, err := wireK8sClient(ctx)
	if err != nil {
		return fmt.Errorf("initializing k8s client: %v", err)
	}

	err = k8sClient.Exec(ctx, k8s.PodID(pod.Name), container.Name(co.Name), k8s.Namespace(pod.Namespace),
		[]string{"which", "bash"}, &bytes.Buffer{}, io.Discard, io.Discard)
	if err == nil {
		cmd := model.Cmd{Argv: []string{"kubectl", "exec", "-it", pod.Name, "-n", pod.Namespace, "-c", co.Name, "--", "bash"}}
		logger.Get(ctx).Infof("Running: %v", cmd)
		return c.execer.Exec(kubectlBinary, cmd.Argv, env)
	}

	err = k8sClient.Exec(ctx, k8s.PodID(pod.Name), container.Name(co.Name), k8s.Namespace(pod.Namespace),
		[]string{"which", "sh"}, &bytes.Buffer{}, io.Discard, io.Discard)
	if err == nil {
		cmd := model.Cmd{Argv: []string{"kubectl", "exec", "-it", pod.Name, "-n", pod.Namespace, "-c", co.Name, "--", "sh"}}
		logger.Get(ctx).Infof("Running: %v", cmd)
		return c.execer.Exec(kubectlBinary, cmd.Argv, env)
	}

	return fmt.Errorf(`could not find bash or sh in container image: %s

We're working on a new debugging tool for dynamically installing
a shell in a minimal container image.

https://hub.docker.com/extensions/docker/labs-k8s-toolkit-extension

To try it out, install the Docker Desktop extension, and run 'tilt shell' again.`, co.Image)
}

func (c *shellCmd) selectContainer(pod *v1alpha1.Pod) (v1alpha1.Container, error) {
	if len(pod.InitContainers) == 0 && len(pod.Containers) == 1 {
		if c.container != "" && pod.Containers[0].Name != c.container {
			return v1alpha1.Container{}, fmt.Errorf("container %s not found in pod %s", c.container, pod.Name)
		}
		return pod.Containers[0], nil
	}

	for _, co := range pod.InitContainers {
		if co.Name == c.container {
			return co, nil
		}
	}
	for _, co := range pod.Containers {
		if co.Name == c.container {
			return co, nil
		}
	}
	return v1alpha1.Container{}, fmt.Errorf("container %s not found in pod %s", c.container, pod.Name)
}

type ShellExecer interface {
	// Checks if the given binary exists in the PATH.
	LookPath(file string) (string, error)

	// Replaces the current process with the given binary.
	Exec(binary string, argv []string, env []string) error
}

type realShellExecer struct{}

func (r realShellExecer) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}
func (r realShellExecer) Exec(binary string, argv []string, env []string) error {
	return syscall.Exec(binary, argv, env)
}

package k8s

import (
	"context"
	"errors"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"

	"github.com/tilt-dev/tilt/internal/container"
)

func (k *K8sClient) Exec(_ context.Context, podID PodID, cName container.Name, n Namespace, cmd []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	req := k.core.RESTClient().Post().
		Resource("pods").
		Namespace(n.String()).
		Name(podID.String()).
		SubResource("exec").
		Param("container", cName.String())
	req.VersionedParams(&corev1.PodExecOptions{
		Container: cName.String(),
		Command:   cmd,
		Stdin:     stdin != nil,
		Stdout:    stdout != nil,
		Stderr:    stderr != nil,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(k.restConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to establish connection: %v", err)
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	})
	if err != nil {
		if err.Error() == "" {
			// Executor::Stream() attempts to decode metav1.Status errors to
			// handle non-zero exit codes from commands; unfortunately, for
			// all _other_ failure cases, the error returned is the `Message`
			// field, which might be empty and there's no way for us to further
			// introspect at this point, so a generic message is the best we
			// can do here
			err = errors.New("unknown server error")
		}
		return fmt.Errorf("failed to execute command: %v", err)
	}
	return nil
}

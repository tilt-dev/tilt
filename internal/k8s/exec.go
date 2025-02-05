package k8s

import (
	"context"
	"errors"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"

	"github.com/tilt-dev/tilt/internal/container"
)

func (k *K8sClient) Exec(ctx context.Context, podID PodID, cName container.Name, n Namespace, cmd []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
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

	spdyExec, err := remotecommand.NewSPDYExecutor(k.restConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create spdy executor: %w", err)
	}
	websocketExec, err := remotecommand.NewWebSocketExecutor(k.restConfig, "GET", req.URL().String())
	if err != nil {
		return fmt.Errorf("failed to create websocket executor: %w", err)
	}

	exec, _ := remotecommand.NewFallbackExecutor(websocketExec, spdyExec, func(err error) bool {
		return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
	})

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
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
			return errors.New("unknown server failure")
		}
		return err
	}
	return nil
}

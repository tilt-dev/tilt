package k8s

import (
	"context"
	"fmt"
	"io"

	"github.com/opentracing/opentracing-go"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"

	"github.com/windmilleng/tilt/internal/container"
)

func (k K8sClient) Exec(ctx context.Context, podID PodID, cName container.Name, n Namespace, cmd []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "k8s-Exec")
	_ = ctx
	span.SetTag("cmd", fmt.Sprintf("%v", cmd))
	defer span.Finish()

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
		return err
	}

	return exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	})
}

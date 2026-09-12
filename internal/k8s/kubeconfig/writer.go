// Package kubeconfig writes the "frozen" kubeconfigs that Tilt injects into
// the commands it shells out to -- local(), custom_build, k8s_custom_deploy --
// so that those commands talk to exactly the cluster Tilt is talking to,
// including any --context or --namespace overrides.
//
// The files go in the CLI run's workspace directory, which deletes them as the
// command exits. See xdg.CLIWorkspace.
package kubeconfig

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/clientcmd/api/latest"

	"github.com/tilt-dev/tilt/internal/xdg"
)

// Subdirectory of the workspace we keep frozen kubeconfigs in.
const clusterDir = "cluster"

type Writer struct {
	workspace *xdg.CLIWorkspace
}

func NewWriter(workspace *xdg.CLIWorkspace) *Writer {
	return &Writer{workspace: workspace}
}

func (w *Writer) WriteFrozenKubeConfig(ctx context.Context, nn types.NamespacedName, config *api.Config) (string, error) {
	config = config.DeepCopy()
	err := api.MinifyConfig(config)
	if err != nil {
		return "", fmt.Errorf("minifying Kubernetes config: %v", err)
	}

	err = api.FlattenConfig(config)
	if err != nil {
		return "", fmt.Errorf("flattening Kubernetes config: %v", err)
	}

	obj, err := latest.Scheme.ConvertToVersion(config, latest.ExternalVersion)
	if err != nil {
		return "", fmt.Errorf("converting Kubernetes config: %v", err)
	}

	printer := printers.YAMLPrinter{}
	path, f, err := w.workspace.OpenFile(ctx, clusterDir, fmt.Sprintf("%s.yml", nn.Name))
	if err != nil {
		return "", fmt.Errorf("storing temp kubeconfigs: %v", err)
	}
	defer func() {
		_ = f.Close()
	}()

	err = printer.PrintObj(obj, f)
	if err != nil {
		return "", fmt.Errorf("writing kubeconfig: %v", err)
	}
	return path, nil
}

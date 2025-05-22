package kubeconfig

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/clientcmd/api/latest"

	"github.com/tilt-dev/tilt/internal/xdg"
	"github.com/tilt-dev/tilt/pkg/model"
)

type Writer struct {
	base          xdg.Base
	filesystem    afero.Fs
	apiServerName model.APIServerName
}

func NewWriter(base xdg.Base, filesystem afero.Fs, apiServerName model.APIServerName) *Writer {
	return &Writer{
		base:          base,
		filesystem:    filesystem,
		apiServerName: apiServerName,
	}
}

func (w *Writer) openFrozenKubeConfigFile(ctx context.Context, nn types.NamespacedName) (string, afero.File, error) {
	path, err := w.base.RuntimeFile(
		filepath.Join(string(w.apiServerName), "cluster", fmt.Sprintf("%s.yml", nn.Name)))
	if err == nil {
		var f afero.File
		f, err = w.filesystem.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err == nil {
			return path, f, nil
		}
	}

	path, err = w.base.StateFile(
		filepath.Join(string(w.apiServerName), "cluster", fmt.Sprintf("%s.yml", nn.Name)))
	if err != nil {
		return "", nil, fmt.Errorf("storing temp kubeconfigs: %v", err)
	}

	f, err := w.filesystem.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return "", nil, fmt.Errorf("storing temp kubeconfigs: %v", err)
	}
	return path, f, nil
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
	path, f, err := w.openFrozenKubeConfigFile(ctx, nn)
	if err != nil {
		return "", err
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

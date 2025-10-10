package k8s

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/k8s/kubeconfig"
	"github.com/tilt-dev/tilt/internal/localexec"
	"github.com/tilt-dev/tilt/pkg/logger"
)

func ProvideDefaultLocalKubeconfigPath(
	globalCtx context.Context,
	writer *kubeconfig.Writer,
	apiConfigOrError APIConfigOrError) localexec.KubeconfigPathOnce {
	apiConfig, err := apiConfigOrError.Config, apiConfigOrError.Error
	if err != nil {
		return func() string {
			return ""
		}
	}

	return sync.OnceValue(func() string {
		path, err := writer.WriteFrozenKubeConfig(globalCtx, types.NamespacedName{
			Name:      "default",
			Namespace: "default",
		}, apiConfig)
		if err != nil {
			logger.Get(globalCtx).Warnf("internal error generating kubeconfig: %v", err)
			return ""
		}
		return path
	})
}

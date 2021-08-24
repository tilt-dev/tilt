package k8s

import (
	"fmt"

	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/value"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// Deserializing discovery strategy from starlark values.
type DiscoveryStrategy v1alpha1.KubernetesDiscoveryStrategy

func (ds *DiscoveryStrategy) Unpack(v starlark.Value) error {
	s, ok := value.AsString(v)
	if !ok {
		return fmt.Errorf("Must be a string. Got: %s", v.Type())
	}

	kdStrategy := v1alpha1.KubernetesDiscoveryStrategy(s)
	if !(kdStrategy == "" ||
		kdStrategy == v1alpha1.KubernetesDiscoveryStrategyDefault ||
		kdStrategy == v1alpha1.KubernetesDiscoveryStrategySelectorsOnly) {
		return fmt.Errorf("Invalid. Must be one of: %q, %q",
			v1alpha1.KubernetesDiscoveryStrategyDefault,
			v1alpha1.KubernetesDiscoveryStrategySelectorsOnly)
	}

	*ds = DiscoveryStrategy(kdStrategy)
	return nil
}

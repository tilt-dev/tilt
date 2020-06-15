package localregistry

import (
	"context"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const ConfigMapName = "local-registry-hosting"
const ConfigMapNamespace = "kube-public"
const ConfigMapField = "localRegistryHosting.v1"

// Check if the cluster advertises support for a local registry.
//
// If no registry advertised, returns an empty struct.
//
// If a registry is advertised but we don't understand the config map,
// returns an error.
func Discover(ctx context.Context, core apiv1.CoreV1Interface) (LocalRegistryHostingV1, error) {
	result := LocalRegistryHostingV1{}

	cfg, err := core.ConfigMaps(ConfigMapNamespace).Get(ctx, ConfigMapName, metav1.GetOptions{})
	if err != nil {
		if errors.IsForbidden(err) {
			// We assume that if a cluster has restricted access to the kube-public
			// namespace, then they likely don't have a local registry, so don't
			// bother returning an error.
			return result, nil
		}
		if errors.IsNotFound(err) {
			return result, nil
		}
		return result, err
	}

	data := cfg.Data
	err = yaml.Unmarshal([]byte(data[ConfigMapField]), &result)
	return result, err
}

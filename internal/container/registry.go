package container

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

var escapeRegex = regexp.MustCompile(`[/:@]`)

func escapeName(s string) string {
	return string(escapeRegex.ReplaceAll([]byte(s), []byte("_")))
}

// IsEmptyRegistry returns true if the object is nil or has no Host.
func IsEmptyRegistry(reg *v1alpha1.RegistryHosting) bool {
	if reg == nil || reg.Host == "" {
		return true
	}
	return false
}

// RegistryFromCluster determines the registry that should be used for pushing
// & pulling Tilt-built images.
//
// If the v1alpha1.Cluster object is not in a healthy state, an error is
// returned.
//
// If the v1alpha1.Cluster object is healthy and provides local registry
// metadata, that will be used.
//
// Otherwise, if the v1alpha1.Cluster object is healthy and does not provide
// local registry metadata but a default registry for the cluster is defined
// (typically via `default_registry` in the Tiltfile), the default registry
// will be used.
//
// As a fallback, an empty registry will be returned, which indicates that _no_
// registry rewriting should occur and Tilt should push and pull images to the
// registry as specified by the configuration ref (e.g. what's passed in to
// `docker_build` or `custom_build`).
func RegistryFromCluster(cluster *v1alpha1.Cluster) (*v1alpha1.RegistryHosting, error) {
	if cluster == nil {
		return nil, nil
	}

	if cluster.Status.Error != "" {
		// if the Cluster has not been initialized, we have not had a chance to
		// read the local cluster info from it yet
		return nil, fmt.Errorf("cluster not ready: %s", cluster.Status.Error)
	}

	if cluster.Status.Registry != nil {
		return cluster.Status.Registry.DeepCopy(), nil
	}

	if cluster.Spec.DefaultRegistry != nil {
		// no local registry is configured for this cluster, so use the default
		return cluster.Spec.DefaultRegistry.DeepCopy(), nil
	}

	return nil, nil
}

// replaceRegistry produces a new image name that is in the specified registry.
// The name might be ugly, favoring uniqueness and simplicity and assuming that the prettiness of ephemeral dev image
// names is not that important.
func replaceRegistry(defaultReg string, rs RefSelector, singleName string) (reference.Named, error) {
	if defaultReg == "" {
		return rs.AsNamedOnly(), nil
	}

	// Sometimes users get confused and put the local registry name in the YAML.
	// No need to replace the registry in that case.
	// https://github.com/tilt-dev/tilt/issues/4911
	if strings.HasPrefix(rs.RefFamiliarName(), fmt.Sprintf("%s/", defaultReg)) {
		return rs.AsNamedOnly(), nil
	}

	// validate the ref produced
	newRefString := ""
	if singleName == "" {
		newRefString = fmt.Sprintf("%s/%s", defaultReg, escapeName(rs.RefFamiliarName()))
	} else {
		newRefString = fmt.Sprintf("%s/%s", defaultReg, singleName)
	}

	newRef, err := reference.ParseNamed(newRefString)
	if err != nil {
		return nil, errors.Wrapf(err, "Error parsing %s after applying default registry %s", newRefString, defaultReg)
	}

	return newRef, nil
}

// ReplaceRegistryForLocalRef returns a new reference using the target local
// registry (as seen from the user).
func ReplaceRegistryForLocalRef(rs RefSelector, reg *v1alpha1.RegistryHosting) (reference.Named, error) {
	var host, singleName string
	if !IsEmptyRegistry(reg) {
		host = reg.Host
		singleName = reg.SingleName
	}
	return replaceRegistry(host, rs, singleName)
}

// ReplaceRegistryForContainerRuntimeRef returns a new reference using the target local
// registry (as seen from the container runtime).
func ReplaceRegistryForContainerRuntimeRef(rs RefSelector, reg *v1alpha1.RegistryHosting) (reference.Named, error) {
	var host, singleName string
	if !IsEmptyRegistry(reg) {
		host = reg.HostFromContainerRuntime
		if host == "" {
			host = reg.Host
		}
		singleName = reg.SingleName
	}
	return replaceRegistry(host, rs, singleName)
}

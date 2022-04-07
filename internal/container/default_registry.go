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

type Registry struct {
	// The Host of a container registry where we can push images. e.g.:
	//   - localhost:32000
	//   - gcr.io/windmill-public-containers
	Host string

	// The prefix we use with image names when referring to them from inside the cluster.
	// In most cases, this is equivalent to Host (the host of the container registry that we push to),
	// but sometimes users will specify a hostFromCluster separately (e.g. using a local registry with KIND:
	// YAMLs will specify the image as `registry:5000/my-img`, so the hostFromCluster will be `registry:5000`).
	hostFromCluster string

	// ECR Image registries work differently than other image registries.
	//
	// The registry takes the form
	// https://aws_account_id.dkr.ecr.region.amazonaws.com
	//
	// And each image name in that registry must be pre-created ಠ_ಠ and assigned IAM permissions.
	// https://aws_account_id.dkr.ecr.region.amazonaws.com/my-repo
	// (They call this a repo).
	//
	// For this reason, some users using ECR prefer to push all images to a single image name.
	//
	// I (Nick) am hoping people use this to create a "personal" dev image repo
	// for each user for dev.
	//
	// People have also suggested having a "image name transform function" that matches
	// the "normal" image name to an existing repo.
	//
	// See:
	// https://docs.aws.amazon.com/AmazonECR/latest/userguide/Repositories.html
	// https://github.com/tilt-dev/tilt/issues/2419
	SingleName string
}

func (r Registry) Empty() bool { return r.Host == "" }

func NewRegistry(host string) (Registry, error) {
	r := Registry{Host: host}
	return r, r.Validate()
}

func MustNewRegistry(host string) Registry {
	r, err := NewRegistry(host)
	if err != nil {
		panic(err)
	}
	return r
}

func NewRegistryWithHostFromCluster(host, fromCluster string) (Registry, error) {
	r := Registry{Host: host, hostFromCluster: fromCluster}
	return r, r.Validate()
}

func MustNewRegistryWithHostFromCluster(host, fromCluster string) Registry {
	r, err := NewRegistryWithHostFromCluster(host, fromCluster)
	if err != nil {
		panic(err)
	}
	return r
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
func RegistryFromCluster(cluster v1alpha1.Cluster) (Registry, error) {
	if cluster.Status.Error != "" {
		// if the Cluster has not been initialized, we have not had a chance to
		// read the local cluster info from it yet
		return Registry{}, fmt.Errorf("cluster not ready: %s", cluster.Status.Error)
	}

	regHosting := cluster.Status.Registry
	if regHosting == nil {
		// no local registry is configured for this cluster, so use the default
		regHosting = cluster.Spec.DefaultRegistry
	}

	if regHosting == nil {
		// there was also no default registry, so use registries as specified
		// in the docker_build + custom_build directives in the Tiltfile
		return Registry{}, nil
	}

	reg, err := NewRegistryWithHostFromCluster(
		regHosting.Host,
		regHosting.HostFromContainerRuntime,
	)
	if err != nil {
		return Registry{}, err
	}
	reg.SingleName = regHosting.SingleName
	return reg, nil
}

func (r Registry) Validate() error {
	if r.Host == "" {
		if r.hostFromCluster != "" {
			return fmt.Errorf("illegal registry: provided hostFromCluster %q without "+
				"providing Host", r.hostFromCluster)
		}
		// Empty registry, nothing to validate
		return nil
	}

	err := validateHost(r.Host)
	if err != nil {
		return errors.Wrapf(err, "validating registry host %q", r.Host)
	}
	if r.hostFromCluster != "" {
		err = validateHost(r.hostFromCluster)
		if err != nil {
			return errors.Wrapf(err, "validating registry hostFromCluster %q", r.hostFromCluster)
		}
	}
	return nil
}

func validateHost(h string) error {
	// NOTE(dmiller): we append a fake path to the domain so that we can try and validate it _during_ Tiltfile execution
	// rather than wait to do it when converting the data to the Engine state.
	// As far as I can tell there's no way in Docker to validate a domain _independently_ from a canonical ref.
	fakeRef := fmt.Sprintf("%s/%s", h, "fake")
	_, err := reference.ParseNamed(fakeRef)
	return err
}

// HostFromCluster returns the registry to be used from within the k8s cluster
// (e.g. in k8s YAML). Returns hostFromCluster, if specified; otherwise the Host.
func (r Registry) HostFromCluster() string {
	if r.hostFromCluster != "" {
		return r.hostFromCluster
	}
	return r.Host
}

// replaceRegistry produces a new image name that is in the specified registry.
// The name might be ugly, favoring uniqueness and simplicity and assuming that the prettiness of ephemeral dev image
// names is not that important.
func (r Registry) replaceRegistry(defaultReg string, rs RefSelector) (reference.Named, error) {
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
	if r.SingleName == "" {
		newRefString = fmt.Sprintf("%s/%s", defaultReg, escapeName(rs.RefFamiliarName()))
	} else {
		newRefString = fmt.Sprintf("%s/%s", defaultReg, r.SingleName)
	}

	newRef, err := reference.ParseNamed(newRefString)
	if err != nil {
		return nil, errors.Wrapf(err, "Error parsing %s after applying default registry %s", newRefString, defaultReg)
	}

	return newRef, nil
}

func (r Registry) ReplaceRegistryForLocalRef(rs RefSelector) (reference.Named, error) {
	return r.replaceRegistry(r.Host, rs)
}

func (r Registry) ReplaceRegistryForClusterRef(rs RefSelector) (reference.Named, error) {
	return r.replaceRegistry(r.HostFromCluster(), rs)
}

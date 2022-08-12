package container

import (
	"context"
	"fmt"
	"path"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// RefSet describes the references for a given image:
//  1. ConfigurationRef: ref as specified in the Tiltfile
//  2. LocalRef(): ref as used outside of the cluster (for Docker etc.)
//  3. ClusterRef(): ref as used inside the cluster (in k8s YAML etc.). Often equivalent to
//     LocalRef, but in some cases they diverge: e.g. when using a local registry with KIND,
//     the image localhost:1234/my-image (localRef) is referenced in the YAML as
//     http://registry/my-image (clusterRef).
type RefSet struct {
	// Ref as specified in Tiltfile; used to match a DockerBuild with
	// corresponding k8s YAML. May contain tags, etc. (Also used as
	// user-facing name for this image.)
	ConfigurationRef RefSelector

	// (Optional) registry to prepend to ConfigurationRef to yield ref to use in update and deploy
	registry *v1alpha1.RegistryHosting
}

func NewRefSet(confRef RefSelector, reg *v1alpha1.RegistryHosting) (RefSet, error) {
	r := RefSet{
		ConfigurationRef: confRef,
		registry:         reg,
	}
	return r, r.Validate()
}

func MustSimpleRefSet(ref RefSelector) RefSet {
	r := RefSet{
		ConfigurationRef: ref,
	}
	if err := r.Validate(); err != nil {
		panic(err)
	}
	return r
}

func RefSetFromImageMap(spec v1alpha1.ImageMapSpec, cluster *v1alpha1.Cluster) (RefSet, error) {
	selector, err := SelectorFromImageMap(spec)
	if err != nil {
		return RefSet{}, fmt.Errorf("validating image: %v", err)
	}

	reg, err := RegistryFromCluster(cluster)
	if err != nil {
		return RefSet{}, fmt.Errorf("determining registry: %v", err)
	}

	refs, err := NewRefSet(selector, reg)
	if err != nil {
		return RefSet{}, fmt.Errorf("applying image %s to registry %s: %v", spec.Selector, reg, err)
	}
	return refs, nil
}

func (rs RefSet) WithoutRegistry() RefSet {
	return MustSimpleRefSet(rs.ConfigurationRef)
}

func (rs RefSet) Registry() *v1alpha1.RegistryHosting {
	if rs.registry == nil {
		return nil
	}
	return rs.registry.DeepCopy()
}

func (rs RefSet) MustWithRegistry(reg *v1alpha1.RegistryHosting) RefSet {
	rs.registry = reg
	err := rs.Validate()
	if err != nil {
		panic(err)
	}
	return rs
}

func (rs RefSet) Validate() error {
	if rs.registry != nil {
		err := rs.registry.Validate(context.TODO())
		if err != nil {
			return errors.Wrapf(err.ToAggregate(), "validating new RefSet with configuration ref %q", rs.ConfigurationRef)
		}
	}
	_, err := ReplaceRegistryForLocalRef(rs.ConfigurationRef, rs.registry)
	if err != nil {
		return errors.Wrapf(err, "validating new RefSet with configuration ref %q", rs.ConfigurationRef)
	}

	_, err = ReplaceRegistryForContainerRuntimeRef(rs.ConfigurationRef, rs.registry)
	if err != nil {
		return errors.Wrapf(err, "validating new RefSet with configuration ref %q", rs.ConfigurationRef)
	}

	return nil
}

// LocalRef returns the ref by which this image is referenced from outside the cluster
// (e.g. by `docker build`, `docker push`, etc.)
func (rs RefSet) LocalRef() reference.Named {
	if IsEmptyRegistry(rs.registry) {
		return rs.ConfigurationRef.AsNamedOnly()
	}
	ref, err := ReplaceRegistryForLocalRef(rs.ConfigurationRef, rs.registry)
	if err != nil {
		// Validation should have caught this before now :-/
		panic(fmt.Sprintf("ERROR deriving LocalRef: %v", err))
	}

	return ref
}

// ClusterRef returns the ref by which this image will be pulled by
// the container runtime in the cluster.
//
// For example, the registry host (that the user/Tilt *push* to) might be
// something like `localhost:1234/foo`, referring to an exposed port from the
// registry Docker container. However, when the container runtime (itself
// generally running within a Docker container), won't see it on localhost,
// and will instead use a reference like `registry:5000/foo`.
//
// If HostFromContainerRuntime is not set on the registry for the RefSet, the
// Host will be used instead. This is common in cases where both the user and
// the container runtime refer to the registry in the same way.
//
// Note that this is specific to the container runtime, which might have its
// own config for the host. The local registry specification allows an
// additional "ClusterFromClusterNetwork" value, which describes a generic way
// for access from within the cluster network (e.g. via cluster provided DNS).
// Within Tilt, this value is NOT used for business logic, so sometimes "cluster
// ref" is used to refer to the container runtime ref. The API types, however,
// include both values and are labeled accurately.
//
// TODO(milas): Rename to ContainerRuntimeRef()
func (rs RefSet) ClusterRef() reference.Named {
	if IsEmptyRegistry(rs.registry) {
		return rs.LocalRef()
	}
	ref, err := ReplaceRegistryForContainerRuntimeRef(rs.ConfigurationRef, rs.registry)
	if err != nil {
		// Validation should have caught this before now :-/
		panic(fmt.Sprintf("ERROR deriving ClusterRef: %v", err))
	}
	return ref
}

// AddTagSuffix tags the references for build/deploy.
//
// In most cases, we will use the tag given as-is.
//
// If we're in the mode where we're pushing to a single image name (for ECR), we'll
// tag it with [escaped-original-name]-[suffix].
func (rs RefSet) AddTagSuffix(suffix string) (TaggedRefs, error) {
	tag := suffix
	if rs.registry != nil && rs.registry.SingleName != "" {
		tag = fmt.Sprintf("%s-%s", escapeName(path.Base(rs.ConfigurationRef.RefFamiliarName())), tag)
	}

	localTagged, err := reference.WithTag(rs.LocalRef(), tag)
	if err != nil {
		return TaggedRefs{}, errors.Wrapf(err, "tagging localRef %s as %s", rs.LocalRef().String(), tag)
	}

	// TODO(maia): maybe TaggedRef should behave like RefSet, where clusterRef is optional
	//   and if not set, the accessor returns LocalRef instead
	clusterTagged, err := reference.WithTag(rs.ClusterRef(), tag)
	if err != nil {
		return TaggedRefs{}, errors.Wrapf(err, "tagging clusterRef %s as %s", rs.ClusterRef().String(), tag)
	}
	return TaggedRefs{
		LocalRef:   localTagged,
		ClusterRef: clusterTagged,
	}, nil
}

// TaggedRefs yielded by an image build
type TaggedRefs struct {
	// LocalRef is the image name + tag as referenced from outside cluster
	// (e.g. by the user or Tilt when pushing images).
	LocalRef reference.NamedTagged
	// ClusterRef is the image name + tag as referenced from the
	// container runtime on the cluster.
	//
	// TODO(milas): Rename to ContainerRuntimeRef
	ClusterRef reference.NamedTagged
}

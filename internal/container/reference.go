package container

import (
	"fmt"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
)

// RefSet describes the references for a given image:
//  1. ConfigurationRef: ref as specified in the Tiltfile
//  2. LocalRef(): ref as used outside of the cluster (for Docker etc.)
//  3. ClusterRef(): ref as used inside the cluster (in k8s YAML etc.). Often equivalent to
//      LocalRef, but in some cases they diverge: e.g. when using a local registry with KIND,
//      the image localhost:1234/my-image (localRef) is referenced in the YAML as
//      http://registry/my-image (clusterRef).
type RefSet struct {
	// Ref as specified in Tiltfile; used to match a DockerBuild with
	// corresponding k8s YAML. May contain tags, etc. (Also used as
	// user-facing name for this image.)
	ConfigurationRef RefSelector

	// (Optional) registry to prepend to ConfigurationRef to yield ref to use in update and deploy
	registry Registry
}

func NewRefSet(confRef RefSelector, reg Registry) (RefSet, error) {
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

func (rs RefSet) WithoutRegistry() RefSet {
	return MustSimpleRefSet(rs.ConfigurationRef)
}

func (rs RefSet) MustWithRegistry(reg Registry) RefSet {
	rs.registry = reg
	err := rs.Validate()
	if err != nil {
		panic(err)
	}
	return rs
}

func (rs RefSet) Validate() error {
	if !rs.registry.Empty() {
		err := rs.registry.Validate()
		if err != nil {
			return errors.Wrapf(err, "validating new RefSet with configuration ref %q", rs.ConfigurationRef)
		}
	}
	_, err := rs.registry.ReplaceRegistryForLocalRef(rs.ConfigurationRef)
	if err != nil {
		return errors.Wrapf(err, "validating new RefSet with configuration ref %q", rs.ConfigurationRef)
	}

	_, err = rs.registry.ReplaceRegistryForClusterRef(rs.ConfigurationRef)
	if err != nil {
		return errors.Wrapf(err, "validating new RefSet with configuration ref %q", rs.ConfigurationRef)
	}

	return nil
}

// LocalRef returns the ref by which this image is referenced from outside the cluster
// (e.g. by `docker build`, `docker push`, etc.)
func (rs RefSet) LocalRef() reference.Named {
	if rs.registry.Empty() {
		return rs.ConfigurationRef.AsNamedOnly()
	}
	ref, err := rs.registry.ReplaceRegistryForLocalRef(rs.ConfigurationRef)
	if err != nil {
		// Validation should have caught this before now :-/
		panic(fmt.Sprintf("ERROR deriving LocalRef: %v", err))
	}

	return ref
}

// ClusterRef returns the ref by which this image is referenced in the cluster.
// In most cases the image's ref from the cluster is the same as its ref locally;
// currently, we only allow these refs to diverge if the user provides a default registry
// with different urls for Host and hostFromCluster.
// If registry.hostFromCluster is not set, we return localRef.
func (rs RefSet) ClusterRef() reference.Named {
	if rs.registry.Empty() {
		return rs.LocalRef()
	}
	ref, err := rs.registry.ReplaceRegistryForClusterRef(rs.ConfigurationRef)
	if err != nil {
		// Validation should have caught this before now :-/
		panic(fmt.Sprintf("ERROR deriving ClusterRef: %v", err))
	}
	return ref
}

// TagRefs tags both of the references used for build/deploy with the given tag.
func (rs RefSet) TagRefs(tag string) (TaggedRefs, error) {
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

// Refs yielded by an image build
type TaggedRefs struct {
	LocalRef   reference.NamedTagged // Image name + tag as referenced from outside cluster
	ClusterRef reference.NamedTagged // Image name + tag as referenced from within cluster
}

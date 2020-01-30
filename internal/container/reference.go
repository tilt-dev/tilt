package container

import (
	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
)

type RefSet struct {
	// Ref as specified in Tiltfile; used to match a DockerBuild with
	// corresponding k8s YAML. May contain tags, etc. (Also used as
	// user-facing name for this image.)
	ConfigurationRef RefSelector

	// Image name as referenced from outside the cluster (in Dockerfile,
	// docker push etc.). This will be the ConfigurationRef stripped of
	// tags and (if applicable) prepended with the DefaultRegistry.
	localRef reference.Named

	// The image name (minus the Tilt tag) as referenced from the cluster (in k8s YAML,
	// etc.) (Often this will be the same as the localRef, but in some cases they diverge:
	// e.g. when using a local registry with KIND, the image localhost:1234/my-image
	// (localRef) is referenced in the YAML as http://registry/my-image (clusterRef).
	// clusterRef is optional; if not provided, we assume localRef == clusterRef.
	clusterRef reference.Named
}

func NewRefSet(confRef RefSelector, localRef, clusterRef reference.Named) RefSet {
	return RefSet{
		ConfigurationRef: confRef,
		localRef:         localRef,
		clusterRef:       clusterRef,
	}
}

// SimpleRefSet makes a ref set for the given selector, assuming that
// ConfigurationRef, localRef, and clusterRef are all equal.
func SimpleRefSet(ref RefSelector) RefSet {
	return RefSet{
		ConfigurationRef: ref,
		localRef:         ref.AsNamedOnly(),
	}
}

func (rs RefSet) WithLocalRef(ref reference.Named) RefSet {
	rs.localRef = ref
	return rs
}

func (rs RefSet) WithClusterRef(ref reference.Named) RefSet {
	rs.clusterRef = ref
	return rs
}

// LocalRef returns the ref by which this image is referenced from outside the cluster
// (e.g. by `docker build`, `docker push`, etc.)
func (rs RefSet) LocalRef() reference.Named {
	// todo: derive from registry, panic on err
	return rs.localRef
}

// ClusterRef returns the ref by which this image is referenced in the cluster.
// If no clusterRef is explicitly set, we return localRef, since in most cases
// the image's ref from the cluster is the same as its ref locally.
func (rs RefSet) ClusterRef() reference.Named {
	if rs.clusterRef == nil {
		return rs.localRef
	}
	return rs.clusterRef
}

// TagRefs tags both of the references used for build/deploy with the given tag.
func (rs RefSet) TagRefs(tag string) (TaggedRefs, error) {
	localTagged, err := reference.WithTag(rs.localRef, tag)
	if err != nil {
		return TaggedRefs{}, errors.Wrapf(err, "tagging localRef %s as %s", rs.localRef.String(), tag)
	}

	// TODO(maia): maybe TaggedRef should behave like RefSet, where clusterRef is optional
	//   and if not set, the accessor returns localRef instead
	clusterTagged, err := reference.WithTag(rs.ClusterRef(), tag)
	if err != nil {
		return TaggedRefs{}, errors.Wrapf(err, "tagging clusterRef %s as %s", rs.clusterRef.String(), tag)
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

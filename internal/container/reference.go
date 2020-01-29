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
	LocalRef reference.Named

	// The image name (minus the Tilt tag) as referenced from the cluster (in k8s YAML,
	// etc.) (Often this will be the same as the LocalRef, but in some cases they diverge:
	// e.g. when using a local registry with KIND, the image localhost:1234/my-image
	// (LocalRef) is referenced in the YAML as http://registry/my-image (clusterRef).
	// clusterRef is optional; if not provided, we assume LocalRef == clusterRef.
	clusterRef reference.Named
}

func NewRefSet(confRef RefSelector, localRef, clusterRef reference.Named) RefSet {
	return RefSet{
		ConfigurationRef: confRef,
		LocalRef:         localRef,
		clusterRef:       clusterRef,
	}
}

// SimpleRefSet makes a ref set for the given selector, assuming that
// ConfigurationRef, LocalRef, and clusterRef are all equal.
func SimpleRefSet(ref RefSelector) RefSet {
	return RefSet{
		ConfigurationRef: ref,
		LocalRef:         ref.AsNamedOnly(),
	}
}

func (rs RefSet) WithClusterRef(ref reference.Named) RefSet {
	rs.clusterRef = ref
	return rs
}

// ClusterRef returns the ref by which this image is referenced in the cluster.
// If no clusterRef is explicitly set, we return LocalRef, since in most cases
// the image's ref from the cluster is the same as its ref locally.
func (rs RefSet) ClusterRef() reference.Named {
	if rs.clusterRef == nil {
		return rs.LocalRef
	}
	return rs.clusterRef
}

// TagRefs tags both of the references used for build/deploy with the given tag.
func (rs RefSet) TagRefs(tag string) (TaggedRefs, error) {
	localTagged, err := reference.WithTag(rs.LocalRef, tag)
	if err != nil {
		return TaggedRefs{}, errors.Wrapf(err, "tagging LocalRef %s as %s", rs.LocalRef.String(), tag)
	}

	// TODO(maia): maybe TaggedRef should behave like RefSet, where clusterRef is optional
	//   and if not set, the accessor returns LocalRef instead
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

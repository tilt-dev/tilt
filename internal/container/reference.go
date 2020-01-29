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
	// (LocalRef) is referenced in the YAML as http://registry/my-image (ClusterRef).
	ClusterRef reference.Named
}

// SimpleRefSet makes a ref set for the given selector, assuming that LocalRef
// and ClusterRef are equal.
func SimpleRefSet(ref RefSelector) RefSet {
	return RefSet{
		ConfigurationRef: ref,
		LocalRef:         ref.AsNamedOnly(),
		ClusterRef:       ref.AsNamedOnly(),
	}
}

func (rs RefSet) TagRefs(tag string) (TaggedRefs, error) {
	localTagged, err := reference.WithTag(rs.LocalRef, tag)
	if err != nil {
		return TaggedRefs{}, errors.Wrapf(err, "tagging LocalRef %s as %s", rs.LocalRef.String(), tag)
	}
	clusterTagged, err := reference.WithTag(rs.ClusterRef, tag)
	if err != nil {
		return TaggedRefs{}, errors.Wrapf(err, "tagging ClusterRef %s as %s", rs.ClusterRef.String(), tag)
	}
	return TaggedRefs{
		LocalRef:   localTagged,
		ClusterRef: clusterTagged,
	}, nil
}

type TaggedRefs struct {
	LocalRef   reference.NamedTagged // Image name + tag as referenced from outside cluster
	ClusterRef reference.NamedTagged // Image name + tag as referenced from within cluster
}

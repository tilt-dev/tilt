package container

import "github.com/docker/distribution/reference"

type RefSet struct {
	// Ref as specified in Tiltfile; used to match a DockerBuild with
	// corresponding k8s YAML. May contain tags, etc. (Also used as
	// user-facing name for this image.)
	ConfigurationRef RefSelector

	// What we name the image that we build. This will be the ConfigurationRef
	// stripped of tags and (if applicable) prepended with the DefaultRegistry.
	BuildRef reference.Named // localhost:123/my-image

	// The image name (minus the Tilt tag) that we inject into YAML or otherwise deploy.
	// (Often this will be the same as the BuildRef, but in some cases they diverge:
	// e.g. when using a local registry with KIND, the image localhost:1234/my-image
	// (BuildRef) is referenced in the YAML as http://registry/my-image (DeployRef).
	DeployRef reference.Named

	// ref from local vs. ref from cluster
}

// SimpleRefSet makes a ref set for the given selector, assuming that BuildRef
// and DeployRef are equal.
func SimpleRefSet(ref RefSelector) RefSet {
	return RefSet{
		ConfigurationRef: ref,
		BuildRef:         ref.AsNamedOnly(),
		DeployRef:        ref.AsNamedOnly(),
	}
}

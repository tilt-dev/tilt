package container

import (
	"fmt"
	"regexp"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
)

var escapeRegex = regexp.MustCompile(`[/:@]`)

func escapeName(s string) string {
	return string(escapeRegex.ReplaceAll([]byte(s), []byte("_")))
}

type Registry struct {
	// The Host of a container registry where we can push images. e.g.:
	//   - localhost:32000
	//   - gcr.io/windmill-public-containers
	PushHost string

	// The prefix we use with image names when attempt to pull them from the registry.
	// In most cases, this is equivalent to PushHost (the host of the container registry that we push to),
	// but sometimes users will specify a pullHost separately (e.g. using a local registry with KIND:
	// YAMLs will specify the image as `registry:5000/my-img`, so the pullHost will be `registry:5000`).
	pullHost string
}

func (r Registry) Empty() bool { return r.PushHost == "" }

func NewRegistry(host string) Registry {
	// TODO(maia): validate
	return Registry{PushHost: host}
}

func NewPushPullRegistry(push, pull string) Registry {
	// TODO(maia): validate
	return Registry{PushHost: push, pullHost: pull}
}

// PullHost returns the pullHost, if specified; otherwise the PushHost.
func (r Registry) PullHost() string {
	if r.pullHost != "" {
		return r.pullHost
	}
	return r.PushHost
}

// replaceRegistry produces a new image name that is in the specified registry.
// The name might be ugly, favoring uniqueness and simplicity and assuming that the prettiness of ephemeral dev image
// names is not that important.
func replaceRegistry(defaultReg string, rs RefSelector) (reference.Named, error) {
	if defaultReg == "" {
		return rs.AsNamedOnly(), nil
	}

	// validate the ref produced
	newNs := fmt.Sprintf("%s/%s", defaultReg, escapeName(rs.RefFamiliarName()))
	newN, err := reference.ParseNamed(newNs)
	if err != nil {
		return nil, errors.Wrapf(err, "Error parsing %s after applying default registry %s", newNs, defaultReg)
	}

	return newN, nil
}

func (r Registry) ReplaceRegistryForLocalRef(rs RefSelector) (reference.Named, error) {
	return replaceRegistry(r.PushHost, rs)
}

func (r Registry) ReplaceRegistryForClusterRef(rs RefSelector) (reference.Named, error) {
	return replaceRegistry(r.PullHost(), rs)
}

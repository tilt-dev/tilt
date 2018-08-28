package containerizedengine

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/docker/cli/internal/pkg/containerized"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/pkg/errors"
)

// GetCurrentEngineVersion determines the current type of engine (image) and version
func (c baseClient) GetCurrentEngineVersion(ctx context.Context) (EngineInitOptions, error) {
	ctx = namespaces.WithNamespace(ctx, engineNamespace)
	ret := EngineInitOptions{}
	currentEngine := CommunityEngineImage
	engine, err := c.GetEngine(ctx)
	if err != nil {
		if err == ErrEngineNotPresent {
			return ret, errors.Wrap(err, "failed to find existing engine")
		}
		return ret, err
	}
	imageName, err := c.getEngineImage(engine)
	if err != nil {
		return ret, err
	}
	distributionRef, err := reference.ParseNormalizedNamed(imageName)
	if err != nil {
		return ret, errors.Wrapf(err, "failed to parse image name: %s", imageName)
	}

	if strings.Contains(distributionRef.Name(), EnterpriseEngineImage) {
		currentEngine = EnterpriseEngineImage
	}
	taggedRef, ok := distributionRef.(reference.NamedTagged)
	if !ok {
		return ret, ErrEngineImageMissingTag
	}
	ret.EngineImage = currentEngine
	ret.EngineVersion = taggedRef.Tag()
	ret.RegistryPrefix = reference.Domain(taggedRef) + "/" + path.Dir(reference.Path(taggedRef))
	return ret, nil
}

// ActivateEngine will switch the image from the CE to EE image
func (c baseClient) ActivateEngine(ctx context.Context, opts EngineInitOptions, out OutStream,
	authConfig *types.AuthConfig, healthfn func(context.Context) error) error {

	// set the proxy scope to "ee" for activate flows
	opts.scope = "ee"

	ctx = namespaces.WithNamespace(ctx, engineNamespace)

	// If version is unspecified, use the existing engine version
	if opts.EngineVersion == "" {
		currentOpts, err := c.GetCurrentEngineVersion(ctx)
		if err != nil {
			return err
		}
		opts.EngineVersion = currentOpts.EngineVersion
		if currentOpts.EngineImage == EnterpriseEngineImage {
			// This is a "no-op" activation so the only change would be the license - don't update the engine itself
			return nil
		}
	}
	return c.DoUpdate(ctx, opts, out, authConfig, healthfn)
}

// DoUpdate performs the underlying engine update
func (c baseClient) DoUpdate(ctx context.Context, opts EngineInitOptions, out OutStream,
	authConfig *types.AuthConfig, healthfn func(context.Context) error) error {

	ctx = namespaces.WithNamespace(ctx, engineNamespace)
	if opts.EngineVersion == "" {
		// TODO - Future enhancement: This could be improved to be
		// smart about figuring out the latest patch rev for the
		// current engine version and automatically apply it so users
		// could stay in sync by simply having a scheduled
		// `docker engine update`
		return fmt.Errorf("please pick the version you want to update to")
	}

	imageName := fmt.Sprintf("%s/%s:%s", opts.RegistryPrefix, opts.EngineImage, opts.EngineVersion)

	// Look for desired image
	image, err := c.cclient.GetImage(ctx, imageName)
	if err != nil {
		if errdefs.IsNotFound(err) {
			image, err = c.pullWithAuth(ctx, imageName, out, authConfig)
			if err != nil {
				return errors.Wrapf(err, "unable to pull image %s", imageName)
			}
		} else {
			return errors.Wrapf(err, "unable to check for image %s", imageName)
		}
	}

	// Gather information about the existing engine so we can recreate it
	engine, err := c.GetEngine(ctx)
	if err != nil {
		if err == ErrEngineNotPresent {
			return errors.Wrap(err, "unable to find existing engine - please use init")
		}
		return err
	}

	// TODO verify the image has changed and don't update if nothing has changed

	err = containerized.AtomicImageUpdate(ctx, engine, image, func() error {
		ctx, cancel := context.WithTimeout(ctx, engineWaitTimeout)
		defer cancel()
		return c.waitForEngine(ctx, out, healthfn)
	})
	if err == nil && opts.scope != "" {
		var labels map[string]string
		labels, err = engine.Labels(ctx)
		if err != nil {
			return err
		}
		labels[proxyLabel] = opts.scope
		_, err = engine.SetLabels(ctx, labels)
	}
	return err
}

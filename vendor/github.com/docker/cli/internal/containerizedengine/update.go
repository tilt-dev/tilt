package containerizedengine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/namespaces"
	"github.com/docker/cli/internal/versions"
	clitypes "github.com/docker/cli/types"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	ver "github.com/hashicorp/go-version"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// ActivateEngine will switch the image from the CE to EE image
func (c *baseClient) ActivateEngine(ctx context.Context, opts clitypes.EngineInitOptions, out clitypes.OutStream,
	authConfig *types.AuthConfig) error {

	// If the user didn't specify an image, determine the correct enterprise image to use
	if opts.EngineImage == "" {
		localMetadata, err := versions.GetCurrentRuntimeMetadata(opts.RuntimeMetadataDir)
		if err != nil {
			return errors.Wrap(err, "unable to determine the installed engine version. Specify which engine image to update with --engine-image")
		}

		engineImage := localMetadata.EngineImage
		if engineImage == clitypes.EnterpriseEngineImage || engineImage == clitypes.CommunityEngineImage {
			opts.EngineImage = clitypes.EnterpriseEngineImage
		} else {
			// Chop off the standard prefix and retain any trailing OS specific image details
			// e.g., engine-community-dm -> engine-enterprise-dm
			engineImage = strings.TrimPrefix(engineImage, clitypes.EnterpriseEngineImage)
			engineImage = strings.TrimPrefix(engineImage, clitypes.CommunityEngineImage)
			opts.EngineImage = clitypes.EnterpriseEngineImage + engineImage
		}
	}

	ctx = namespaces.WithNamespace(ctx, engineNamespace)
	return c.DoUpdate(ctx, opts, out, authConfig)
}

// DoUpdate performs the underlying engine update
func (c *baseClient) DoUpdate(ctx context.Context, opts clitypes.EngineInitOptions, out clitypes.OutStream,
	authConfig *types.AuthConfig) error {

	ctx = namespaces.WithNamespace(ctx, engineNamespace)
	if opts.EngineVersion == "" {
		// TODO - Future enhancement: This could be improved to be
		// smart about figuring out the latest patch rev for the
		// current engine version and automatically apply it so users
		// could stay in sync by simply having a scheduled
		// `docker engine update`
		return fmt.Errorf("pick the version you want to update to with --version")
	}
	var localMetadata *clitypes.RuntimeMetadata
	if opts.EngineImage == "" {
		var err error
		localMetadata, err = versions.GetCurrentRuntimeMetadata(opts.RuntimeMetadataDir)
		if err != nil {
			return errors.Wrap(err, "unable to determine the installed engine version. Specify which engine image to update with --engine-image set to 'engine-community' or 'engine-enterprise'")
		}
		opts.EngineImage = localMetadata.EngineImage
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

	// Make sure we're safe to proceed
	newMetadata, err := c.PreflightCheck(ctx, image)
	if err != nil {
		return err
	}
	if localMetadata != nil {
		if localMetadata.Platform != newMetadata.Platform {
			fmt.Fprintf(out, "\nNotice: you have switched to \"%s\".  Refer to %s for update instructions.\n\n", newMetadata.Platform, getReleaseNotesURL(imageName))
		}
	}

	if err := c.cclient.Install(ctx, image, containerd.WithInstallReplace, containerd.WithInstallPath("/usr")); err != nil {
		return err
	}

	return versions.WriteRuntimeMetadata(opts.RuntimeMetadataDir, newMetadata)
}

// PreflightCheck verifies the specified image is compatible with the local system before proceeding to update/activate
// If things look good, the RuntimeMetadata for the new image is returned and can be written out to the host
func (c *baseClient) PreflightCheck(ctx context.Context, image containerd.Image) (*clitypes.RuntimeMetadata, error) {
	var metadata clitypes.RuntimeMetadata
	ic, err := image.Config(ctx)
	if err != nil {
		return nil, err
	}
	var (
		ociimage v1.Image
		config   v1.ImageConfig
	)
	switch ic.MediaType {
	case v1.MediaTypeImageConfig, images.MediaTypeDockerSchema2Config:
		p, err := content.ReadBlob(ctx, image.ContentStore(), ic)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(p, &ociimage); err != nil {
			return nil, err
		}
		config = ociimage.Config
	default:
		return nil, fmt.Errorf("unknown image %s config media type %s", image.Name(), ic.MediaType)
	}

	metadataString, ok := config.Labels["com.docker."+clitypes.RuntimeMetadataName]
	if !ok {
		return nil, fmt.Errorf("image %s does not contain runtime metadata label %s", image.Name(), clitypes.RuntimeMetadataName)
	}
	err = json.Unmarshal([]byte(metadataString), &metadata)
	if err != nil {
		return nil, errors.Wrapf(err, "malformed runtime metadata file in %s", image.Name())
	}

	// Current CLI only supports host install runtime
	if metadata.Runtime != "host_install" {
		return nil, fmt.Errorf("unsupported daemon image: %s\nConsult the release notes at %s for upgrade instructions", metadata.Runtime, getReleaseNotesURL(image.Name()))
	}

	// Verify local containerd is new enough
	localVersion, err := c.cclient.Version(ctx)
	if err != nil {
		return nil, err
	}
	if metadata.ContainerdMinVersion != "" {
		lv, err := ver.NewVersion(localVersion.Version)
		if err != nil {
			return nil, err
		}
		mv, err := ver.NewVersion(metadata.ContainerdMinVersion)
		if err != nil {
			return nil, err
		}
		if lv.LessThan(mv) {
			return nil, fmt.Errorf("local containerd is too old: %s - this engine version requires %s or newer.\nConsult the release notes at %s for upgrade instructions",
				localVersion.Version, metadata.ContainerdMinVersion, getReleaseNotesURL(image.Name()))
		}
	} // If omitted on metadata, no hard dependency on containerd version beyond 18.09 baseline

	// All checks look OK, proceed with update
	return &metadata, nil
}

// getReleaseNotesURL returns a release notes url
// If the image name does not contain a version tag, the base release notes URL is returned
func getReleaseNotesURL(imageName string) string {
	versionTag := ""
	distributionRef, err := reference.ParseNormalizedNamed(imageName)
	if err == nil {
		taggedRef, ok := distributionRef.(reference.NamedTagged)
		if ok {
			versionTag = taggedRef.Tag()
		}
	}
	return fmt.Sprintf("%s/%s", clitypes.ReleaseNotePrefix, versionTag)
}

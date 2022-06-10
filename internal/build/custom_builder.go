package build

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	ktypes "k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/apis/imagemap"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type CustomBuilder struct {
	dCli  docker.Client
	clock Clock
}

func NewCustomBuilder(dCli docker.Client, clock Clock) *CustomBuilder {
	return &CustomBuilder{
		dCli:  dCli,
		clock: clock,
	}
}

func (b *CustomBuilder) Build(ctx context.Context, refs container.RefSet,
	spec v1alpha1.CmdImageSpec,
	imageMaps map[ktypes.NamespacedName]*v1alpha1.ImageMap) (container.TaggedRefs, error) {
	expectedTag := spec.OutputTag
	outputsImageRefTo := spec.OutputsImageRefTo
	var registryHost string
	reg := refs.Registry()
	if reg != nil {
		registryHost = reg.Host
	}

	var expectedBuildRefs container.TaggedRefs
	var err error

	// There are 3 modes for determining the output tag.
	if outputsImageRefTo != "" {
		// In outputs_image_ref_to mode, the user script MUST print the tag to a file,
		// which we recover later. So no need to set expectedBuildRefs.

		// Remove the output file, ignoring any errors.
		_ = os.Remove(outputsImageRefTo)
	} else if expectedTag != "" {
		// If the tag is coming from the user script, we expect that the user script
		// also doesn't know about the local registry. So we have to strip off
		// the registry, and re-add it later.
		expectedBuildRefs, err = refs.WithoutRegistry().AddTagSuffix(expectedTag)
		if err != nil {
			return container.TaggedRefs{}, errors.Wrap(err, "custom_build")
		}
	} else {
		// In "normal" mode, the user's script should use whichever registry tag we give it.
		expectedBuildRefs, err = refs.AddTagSuffix(fmt.Sprintf("tilt-build-%d", b.clock.Now().Unix()))
		if err != nil {
			return container.TaggedRefs{}, errors.Wrap(err, "custom_build")
		}
	}

	expectedBuildResult := expectedBuildRefs.LocalRef

	// TODO(nick): Use the Cmd API.
	// TODO(nick): Inject the image maps into the command environment.
	cmd := exec.CommandContext(ctx, spec.Args[0], spec.Args[1:]...)
	cmd.Dir = spec.Dir
	cmd.Env = logger.DefaultEnv(ctx)

	l := logger.Get(ctx)

	extraEnvVars := []string{}
	if expectedBuildResult != nil {
		extraEnvVars = append(extraEnvVars,
			fmt.Sprintf("EXPECTED_REF=%s", container.FamiliarString(expectedBuildResult)))
		extraEnvVars = append(extraEnvVars,
			fmt.Sprintf("EXPECTED_IMAGE=%s", reference.Path(expectedBuildResult)))
		extraEnvVars = append(extraEnvVars,
			fmt.Sprintf("EXPECTED_TAG=%s", expectedBuildResult.Tag()))
	}
	if registryHost != "" {
		// kept for backwards compatibility
		extraEnvVars = append(extraEnvVars,
			fmt.Sprintf("REGISTRY_HOST=%s", registryHost))
		// for consistency with other EXPECTED_* vars
		extraEnvVars = append(extraEnvVars,
			fmt.Sprintf("EXPECTED_REGISTRY=%s", registryHost))
	}

	extraEnvVars = append(extraEnvVars, b.dCli.Env().AsEnviron()...)

	if len(extraEnvVars) == 0 {
		l.Infof("Custom Build:")
	} else {
		l.Infof("Custom Build: Injecting Environment Variables")
		for _, v := range extraEnvVars {
			l.Infof("  %s", v)
		}
	}
	cmd.Env = append(os.Environ(), extraEnvVars...)
	err = imagemap.InjectIntoLocalEnv(cmd, spec.ImageMaps, imageMaps)
	if err != nil {
		return container.TaggedRefs{}, errors.Wrap(err, "custom_build")
	}

	w := l.Writer(logger.InfoLvl)
	cmd.Stdout = w
	cmd.Stderr = w

	l.Infof("Running custom build cmd %q", model.Cmd{Argv: spec.Args}.String())
	err = cmd.Run()
	if err != nil {
		return container.TaggedRefs{}, errors.Wrap(err, "Custom build command failed")
	}

	if outputsImageRefTo != "" {
		expectedBuildRefs, err = b.readImageRef(ctx, outputsImageRefTo, reg)
		if err != nil {
			return container.TaggedRefs{}, err
		}
		expectedBuildResult = expectedBuildRefs.LocalRef
	}

	// If the command skips the local docker registry, then we don't expect the image
	// to be available (because the command has its own registry).
	if spec.OutputMode == v1alpha1.CmdImageOutputRemote {
		return expectedBuildRefs, nil
	}

	inspect, _, err := b.dCli.ImageInspectWithRaw(ctx, expectedBuildResult.String())
	if err != nil {
		return container.TaggedRefs{}, errors.Wrap(err, "Could not find image in Docker\n"+
			"Did your custom_build script properly tag the image?\n"+
			"If your custom_build doesn't use Docker, you might need to use skips_local_docker=True, "+
			"see https://docs.tilt.dev/custom_build.html\n")
	}

	if outputsImageRefTo != "" {
		// If we're using a custom_build-determined build ref, we don't use content-based tags.
		return expectedBuildRefs, nil
	}

	dig := digest.Digest(inspect.ID)

	tag, err := digestAsTag(dig)
	if err != nil {
		return container.TaggedRefs{}, errors.Wrap(err, "custom_build")
	}

	taggedWithDigest, err := refs.AddTagSuffix(tag)
	if err != nil {
		return container.TaggedRefs{}, errors.Wrap(err, "custom_build")
	}

	// Docker client only needs to care about the localImage
	err = b.dCli.ImageTag(ctx, dig.String(), taggedWithDigest.LocalRef.String())
	if err != nil {
		return container.TaggedRefs{}, errors.Wrap(err, "custom_build")
	}

	return taggedWithDigest, nil
}

func (b *CustomBuilder) readImageRef(ctx context.Context, outputsImageRefTo string, reg *v1alpha1.RegistryHosting) (container.TaggedRefs, error) {
	contents, err := ioutil.ReadFile(outputsImageRefTo)
	if err != nil {
		return container.TaggedRefs{}, fmt.Errorf("Could not find image ref in output. Your custom_build script should have written to %s: %v", outputsImageRefTo, err)
	}

	refStr := strings.TrimSpace(string(contents))
	ref, err := container.ParseNamedTagged(refStr)
	if err != nil {
		return container.TaggedRefs{}, fmt.Errorf("Output image ref in file %s was invalid: %v",
			outputsImageRefTo, err)
	}

	clusterRef := ref
	if reg != nil && reg.HostFromContainerRuntime != "" {
		replacedName, err := container.ParseNamed(strings.Replace(ref.Name(), reg.Host, reg.HostFromContainerRuntime, 1))
		if err != nil {
			return container.TaggedRefs{}, fmt.Errorf("Error converting image ref for cluster: %w", err)
		}
		clusterRef, err = reference.WithTag(replacedName, ref.Tag())
		if err != nil {
			return container.TaggedRefs{}, fmt.Errorf("Error converting image ref for cluster: %w", err)
		}
	}

	return container.TaggedRefs{
		LocalRef:   ref,
		ClusterRef: clusterRef,
	}, nil
}

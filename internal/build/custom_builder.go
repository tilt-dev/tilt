package build

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type CustomBuilder interface {
	Build(ctx context.Context, refs container.RefSet, cb model.CustomBuild) (container.TaggedRefs, error)
}

type ExecCustomBuilder struct {
	dCli  docker.Client
	clock Clock
}

func NewExecCustomBuilder(dCli docker.Client, clock Clock) *ExecCustomBuilder {
	return &ExecCustomBuilder{
		dCli:  dCli,
		clock: clock,
	}
}

func (b *ExecCustomBuilder) Build(ctx context.Context, refs container.RefSet, cb model.CustomBuild) (container.TaggedRefs, error) {
	workDir := cb.WorkDir
	expectedTag := cb.Tag
	command := cb.Command

	skipsLocalDocker := cb.SkipsLocalDocker
	outputsImageRefTo := cb.OutputsImageRefTo

	var expectedBuildRefs container.TaggedRefs
	var registryHost string
	var err error

	// There are 3 modes for determining the output tag.
	if outputsImageRefTo != "" {
		// In outputs_image_ref_to mode, the user script MUST print the tag to a file,
		// which we recover later. So no need to set expectedBuildRefs.

		// Remove the output file, ignoring any errors.
		_ = os.Remove(outputsImageRefTo)

		// Inform the user script about the registry host
		registryHost = refs.Registry().Host

	} else if expectedTag != "" {
		// If the tag is coming from the user script, we expect that the user script
		// also doesn't know about the local registry. So we have to strip off
		// the registry, and re-add it later.
		expectedBuildRefs, err = refs.WithoutRegistry().AddTagSuffix(expectedTag)
		if err != nil {
			return container.TaggedRefs{}, errors.Wrap(err, "CustomBuilder.Build")
		}
	} else {
		// In "normal" mode, the user's script should use whichever registry tag we give it.
		expectedBuildRefs, err = refs.AddTagSuffix(fmt.Sprintf("tilt-build-%d", b.clock.Now().Unix()))
		if err != nil {
			return container.TaggedRefs{}, errors.Wrap(err, "CustomBuilder.Build")
		}
	}

	expectedBuildResult := expectedBuildRefs.LocalRef

	cmd := exec.CommandContext(ctx, command.Argv[0], command.Argv[1:]...)
	cmd.Dir = workDir

	l := logger.Get(ctx)

	extraEnvVars := []string{}
	if expectedBuildResult != nil {
		extraEnvVars = append(extraEnvVars,
			fmt.Sprintf("EXPECTED_REF=%s", container.FamiliarString(expectedBuildResult)))
	}
	if registryHost != "" {
		extraEnvVars = append(extraEnvVars,
			fmt.Sprintf("REGISTRY_HOST=%s", registryHost))
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

	w := l.Writer(logger.InfoLvl)
	cmd.Stdout = w
	cmd.Stderr = w

	l.Infof("Running custom build cmd %q", command)
	err = cmd.Run()
	if err != nil {
		return container.TaggedRefs{}, errors.Wrap(err, "Custom build command failed")
	}

	if outputsImageRefTo != "" {
		expectedBuildRefs, err = b.readImageRef(ctx, outputsImageRefTo)
		if err != nil {
			return container.TaggedRefs{}, err
		}
		expectedBuildResult = expectedBuildRefs.LocalRef
	}

	// If the command skips the local docker registry, then we don't expect the image
	// to be available (because the command has its own registry).
	if skipsLocalDocker {
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
		return container.TaggedRefs{}, errors.Wrap(err, "CustomBuilder.Build")
	}

	taggedWithDigest, err := refs.AddTagSuffix(tag)
	if err != nil {
		return container.TaggedRefs{}, errors.Wrap(err, "CustomBuilder.Build")
	}

	// Docker client only needs to care about the localImage
	err = b.dCli.ImageTag(ctx, dig.String(), taggedWithDigest.LocalRef.String())
	if err != nil {
		return container.TaggedRefs{}, errors.Wrap(err, "CustomBuilder.Build")
	}

	return taggedWithDigest, nil
}

func (b *ExecCustomBuilder) readImageRef(ctx context.Context, outputsImageRefTo string) (container.TaggedRefs, error) {
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

	// TODO(nick): Add support for separate local and cluster refs.
	return container.TaggedRefs{
		LocalRef:   ref,
		ClusterRef: ref,
	}, nil
}

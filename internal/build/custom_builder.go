package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
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

	if expectedTag == "" {
		expectedTag = fmt.Sprintf("tilt-build-%d", b.clock.Now().Unix())
	}

	expectedRefs, err := refs.TagRefs(expectedTag)
	if err != nil {
		return container.TaggedRefs{}, errors.Wrap(err, "CustomBuilder.Build")
	}
	expectedLocal := expectedRefs.LocalRef

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = workDir

	l := logger.Get(ctx)
	l.Infof("Custom Build: Injecting Environment Variables")
	l.Infof("EXPECTED_REF=%s", container.FamiliarString(expectedLocal))
	env := append(os.Environ(), fmt.Sprintf("EXPECTED_REF=%s", container.FamiliarString(expectedLocal)))

	for _, e := range b.dCli.Env().AsEnviron() {
		env = append(env, e)
		l.Infof("%s", e)
	}
	cmd.Env = env

	w := l.Writer(logger.InfoLvl)
	cmd.Stdout = w
	cmd.Stderr = w

	l.Infof("Running custom build cmd %q", command)
	err = cmd.Run()
	if err != nil {
		return container.TaggedRefs{}, errors.Wrap(err, "Custom build command failed")
	}

	// If the command skips the local docker registry, then we don't expect the image
	// to be available (because the command has its own registry).
	if skipsLocalDocker {
		return expectedRefs, nil
	}

	inspect, _, err := b.dCli.ImageInspectWithRaw(ctx, expectedLocal.String())
	if err != nil {
		return container.TaggedRefs{}, errors.Wrap(err, "Could not find image in Docker\n"+
			"If your custom_build doesn't use Docker, you might need to use skips_local_docker=True, "+
			"see https://docs.tilt.dev/custom_build.html\n")
	}

	dig := digest.Digest(inspect.ID)

	tag, err := digestAsTag(dig)
	if err != nil {
		return container.TaggedRefs{}, errors.Wrap(err, "CustomBuilder.Build")
	}

	taggedWithDigest, err := refs.TagRefs(tag)
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

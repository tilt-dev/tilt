package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/docker/distribution/reference"
	"github.com/opencontainers/go-digest"

	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/logger"
)

type CustomBuilder interface {
	Build(ctx context.Context, ref reference.Named, command string) (reference.NamedTagged, error)
}

type ExecCustomBuilder struct {
	dCli  docker.Client
	env   docker.Env
	clock Clock
}

func NewExecCustomBuilder(dCli docker.Client, env docker.Env, clock Clock) *ExecCustomBuilder {
	return &ExecCustomBuilder{
		dCli:  dCli,
		env:   env,
		clock: clock,
	}
}

// NOTE(maia): `Build` takes the ConfigurationRef, i.e. the ref the user originally configured;
// if this ref has a tag, we assume that the output of the build will have the same tag.
// (i.e. if user configures gcr.io/myimage:mytag, we check build output for gcr.io/myimage:mytag. If
// user configures just gcr.io/myimage, we generate an expected tag and check build
// output for gcr.io/myimage:generatedtag
func (b *ExecCustomBuilder) Build(ctx context.Context, ref reference.Named, command string) (reference.NamedTagged, error) {
	l := logger.Get(ctx)
	var err error

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	env := os.Environ()

	withTag, ok := ref.(reference.NamedTagged)
	if !ok {
		// ref-to-build doesn't have a tag; generate a tmp tag to build it with
		// (so we can check afterwards that the correct image was created)
		tmpTag := fmt.Sprintf("tilt-build-%d", b.clock.Now().Unix())
		withTag, err = reference.WithTag(ref, tmpTag)
		if err != nil {
			return nil, err
		}
		l.Infof("TAG=%s", withTag.String())
		env = append(env, fmt.Sprintf("TAG=%s", withTag.String()))
	}

	l.Infof("Custom Build: Injecting Environment Variables")
	for _, e := range b.env.AsEnviron() {
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
		return nil, err
	}

	inspect, _, err := b.dCli.ImageInspectWithRaw(ctx, withTag.String())
	if err != nil {
		return nil, err
	}

	dig := digest.Digest(inspect.ID)

	tag, err := digestAsTag(dig)
	if err != nil {
		return nil, err
	}

	namedTagged, err := reference.WithTag(ref, tag)
	if err != nil {
		return nil, err
	}

	err = b.dCli.ImageTag(ctx, dig.String(), namedTagged.String())
	if err != nil {
		return nil, err
	}

	return namedTagged, nil
}

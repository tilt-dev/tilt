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
	env   []string
	clock Clock
}

func NewExecCustomBuilder(dCli docker.Client, env docker.Env, clock Clock) *ExecCustomBuilder {
	return &ExecCustomBuilder{
		dCli:  dCli,
		env:   env.BuildInjections,
		clock: clock,
	}
}

func (b *ExecCustomBuilder) Build(ctx context.Context, ref reference.Named, command string) (reference.NamedTagged, error) {
	tmpTag := fmt.Sprintf("tilt-build-%d", b.clock.Now().Unix())
	result, err := reference.WithTag(ref, tmpTag)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", command)

	l := logger.Get(ctx)
	l.Infof("Custom Build: Injecting Environment Variables")
	l.Infof("TAG=%s", result.String())
	env := append(os.Environ(), fmt.Sprintf("TAG=%s", result.String()))
	for _, e := range b.env {
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

	inspect, _, err := b.dCli.ImageInspectWithRaw(ctx, result.String())
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

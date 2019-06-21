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
	Build(ctx context.Context, ref reference.Named, command string, expectedTag string) (reference.NamedTagged, error)
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

func (b *ExecCustomBuilder) Build(ctx context.Context, ref reference.Named, command string, expectedTag string) (reference.NamedTagged, error) {
	if expectedTag == "" {
		expectedTag = fmt.Sprintf("tilt-build-%d", b.clock.Now().Unix())
	}

	expectedRef, err := reference.WithTag(ref, expectedTag)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", command)

	l := logger.Get(ctx)
	l.Infof("Custom Build: Injecting Environment Variables")
	l.Infof("EXPECTED_REF=%s", expectedRef.String())
	env := append(os.Environ(), fmt.Sprintf("EXPECTED_REF=%s", expectedRef.String()))

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
		return nil, err
	}

	inspect, _, err := b.dCli.ImageInspectWithRaw(ctx, expectedRef.String())
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

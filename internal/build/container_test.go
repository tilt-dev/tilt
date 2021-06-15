//+build !skipcontainertests,!windows

// Tests that involve spinning up/interacting with actual containers
package build

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockerfile"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

// * * * IMAGE BUILDER * * *

func TestDockerBuildDockerfile(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	df := dockerfile.Dockerfile(`
FROM alpine
WORKDIR /src
ADD a.txt .
RUN cp a.txt b.txt
ADD dir/c.txt .
`)

	f.WriteFile("a.txt", "a")
	f.WriteFile("dir/c.txt", "c")
	f.WriteFile("missing.txt", "missing")

	refs, err := f.b.BuildImage(f.ctx, f.ps, f.getNameFromTest(), model.DockerBuild{
		Dockerfile: df.String(),
		BuildPath:  f.Path(),
	}, model.EmptyMatcher)
	if err != nil {
		t.Fatal(err)
	}

	f.assertImageHasLabels(refs.LocalRef, docker.BuiltByTiltLabel)

	pcs := []expectedFile{
		expectedFile{Path: "/src/a.txt", Contents: "a"},
		expectedFile{Path: "/src/b.txt", Contents: "a"},
		expectedFile{Path: "/src/c.txt", Contents: "c"},
		expectedFile{Path: "/src/dir/c.txt", Missing: true},
		expectedFile{Path: "/src/missing.txt", Missing: true},
	}
	f.assertFilesInImage(refs.LocalRef, pcs)
}

func TestDockerBuildWithBuildArgs(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	df := dockerfile.Dockerfile(`FROM alpine
ARG some_variable_name

ADD $some_variable_name /test.txt`)

	f.WriteFile("awesome_variable", "hi im an awesome variable")

	ba := model.DockerBuildArgs{
		"some_variable_name": "awesome_variable",
	}
	refs, err := f.b.BuildImage(f.ctx, f.ps, f.getNameFromTest(), model.DockerBuild{
		Dockerfile: df.String(),
		BuildPath:  f.Path(),
		BuildArgs:  ba,
	}, model.EmptyMatcher)
	if err != nil {
		t.Fatal(err)
	}

	expected := []expectedFile{
		expectedFile{Path: "/test.txt", Contents: "hi im an awesome variable"},
	}
	f.assertFilesInImage(refs.LocalRef, expected)
}

func TestDockerBuildWithExtraTags(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	df := dockerfile.Dockerfile(`
FROM alpine
WORKDIR /src
ADD a.txt .`)

	f.WriteFile("a.txt", "a")

	refs, err := f.b.BuildImage(f.ctx, f.ps, f.getNameFromTest(), model.DockerBuild{
		Dockerfile: df.String(),
		BuildPath:  f.Path(),
		ExtraTags:  []string{"fe:jenkins-1234"},
	}, model.EmptyMatcher)
	if err != nil {
		t.Fatal(err)
	}

	f.assertImageHasLabels(refs.LocalRef, docker.BuiltByTiltLabel)

	pcs := []expectedFile{
		expectedFile{Path: "/src/a.txt", Contents: "a"},
	}
	f.assertFilesInImage(container.MustParseNamedTagged("fe:jenkins-1234"), pcs)
}

func TestDetectBuildkitCorruption(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	out := bytes.NewBuffer(nil)
	ctx := logger.WithLogger(context.Background(), logger.NewTestLogger(out))
	ps := NewPipelineState(ctx, 1, ProvideClock())
	_, err := f.b.BuildImage(ctx, ps, f.getNameFromTest(), model.DockerBuild{
		// Simulate buildkit corruption
		Dockerfile: `FROM alpine
RUN echo 'failed to create LLB definition: failed commit on ref "unknown-sha256:b72fa303a3a5fbf52c723bfcfb93948bb53b3d7e8d22418e9d171a27ad7dcd84": "unknown-sha256:b72fa303a3a5fbf52c723bfcfb93948bb53b3d7e8d22418e9d171a27ad7dcd84" failed size validation: 80941 != 80929: failed precondition' && exit 1
`,
		BuildPath: f.Path(),
	}, model.EmptyMatcher)
	assert.Error(t, err)
	assert.Contains(t, out.String(), "Detected Buildkit corruption. Rebuilding without Buildkit")
	assert.Contains(t, out.String(), "[1/2] FROM docker.io/library/alpine") // buildkit-style output
	assert.Contains(t, out.String(), "Step 1/3 : FROM alpine")              // Legacy output
}

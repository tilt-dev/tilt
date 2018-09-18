package build

import (
	"archive/tar"
	"bytes"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/dockerignore"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils"
)

const simpleDockerfile = Dockerfile("FROM alpine")

func TestDigestAsTag(t *testing.T) {
	dig := digest.Digest("sha256:cc5f4c463f81c55183d8d737ba2f0d30b3e6f3670dbe2da68f0aac168e93fbb1")
	tag, err := digestAsTag(dig)
	if err != nil {
		t.Fatal(err)
	}

	expected := "tilt-cc5f4c463f81c551"
	if tag != expected {
		t.Errorf("Expected %s, actual: %s", expected, tag)
	}
}

func TestDigestMatchesRef(t *testing.T) {
	dig := digest.Digest("sha256:cc5f4c463f81c55183d8d737ba2f0d30b3e6f3670dbe2da68f0aac168e93fbb1")
	tag, err := digestAsTag(dig)
	if err != nil {
		t.Fatal(err)
	}

	ref, _ := k8s.ParseNamedTagged("windmill.build/image:" + tag)
	if !digestMatchesRef(ref, dig) {
		t.Errorf("Expected digest %s to match ref %s", dig, ref)
	}
}

func TestDigestNotMatchesRef(t *testing.T) {
	dig := digest.Digest("sha256:cc5f4c463f81c55183d8d737ba2f0d30b3e6f3670dbe2da68f0aac168e93fbb1")
	ref, _ := k8s.ParseNamedTagged("windmill.build/image:tilt-deadbeef")
	if digestMatchesRef(ref, dig) {
		t.Errorf("Expected digest %s to not match ref %s", dig, ref)
	}
}

func TestDigestAsTagToShort(t *testing.T) {
	dig := digest.Digest("sha256:cc")
	_, err := digestAsTag(dig)
	expected := "too short"
	if err == nil || !strings.Contains(err.Error(), expected) {
		t.Errorf("expected error %q, actual: %v", expected, err)
	}
}

func TestDigestFromSingleStepOutput(t *testing.T) {
	f := newFakeDockerBuildFixture(t)
	defer f.teardown()

	input := docker.ExampleBuildOutput1
	expected := digest.Digest("sha256:11cd0b38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	actual, err := f.b.getDigestFromBuildOutput(f.ctx, bytes.NewBuffer([]byte(input)))
	if err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func TestDigestFromPushOutput(t *testing.T) {
	f := newFakeDockerBuildFixture(t)
	defer f.teardown()

	input := docker.ExamplePushOutput1
	expected := digest.Digest("sha256:cc5f4c463f81c55183d8d737ba2f0d30b3e6f3670dbe2da68f0aac168e93fbb1")
	actual, err := f.b.getDigestFromPushOutput(f.ctx, bytes.NewBuffer([]byte(input)))
	if err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func TestConditionalRunInFakeDocker(t *testing.T) {
	f := newFakeDockerBuildFixture(t)
	defer f.teardown()

	f.WriteFile("a.txt", "a")
	f.WriteFile("b.txt", "b")

	m := model.Mount{
		LocalPath:     f.Path(),
		ContainerPath: "/src",
	}
	inputs, _ := dockerignore.NewDockerPatternMatcher(f.Path(), []string{"a.txt"})
	step1 := model.Step{
		Cmd:     model.ToShellCmd("cat /src/a.txt > /src/c.txt"),
		Trigger: inputs,
	}
	step2 := model.Step{
		Cmd: model.ToShellCmd("cat /src/b.txt > /src/d.txt"),
	}

	_, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), simpleDockerfile, []model.Mount{m}, []model.Step{step1, step2}, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	expected := expectedFile{
		Path: "Dockerfile",
		Contents: `FROM alpine
LABEL "tilt.buildMode"="scratch"
LABEL "tilt.test"="1"
COPY /src/a.txt /src/a.txt
RUN cat /src/a.txt > /src/c.txt
ADD . /
RUN cat /src/b.txt > /src/d.txt`,
	}
	testutils.AssertFileInTar(f.t, tar.NewReader(f.fakeDocker.BuildOptions.Context), expected)
}

func TestAllConditionalRunsInFakeDocker(t *testing.T) {
	f := newFakeDockerBuildFixture(t)
	defer f.teardown()

	f.WriteFile("a.txt", "a")
	f.WriteFile("b.txt", "b")

	m := model.Mount{
		LocalPath:     f.Path(),
		ContainerPath: "/src",
	}
	inputs, _ := dockerignore.NewDockerPatternMatcher(f.Path(), []string{"a.txt"})
	step1 := model.Step{
		Cmd:     model.ToShellCmd("cat /src/a.txt > /src/c.txt"),
		Trigger: inputs,
	}

	_, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), simpleDockerfile, []model.Mount{m}, []model.Step{step1}, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	expected := expectedFile{
		Path: "Dockerfile",
		Contents: `FROM alpine
LABEL "tilt.buildMode"="scratch"
LABEL "tilt.test"="1"
COPY /src/a.txt /src/a.txt
RUN cat /src/a.txt > /src/c.txt
ADD . /`,
	}
	testutils.AssertFileInTar(f.t, tar.NewReader(f.fakeDocker.BuildOptions.Context), expected)
}

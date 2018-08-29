package build

import (
	"bytes"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
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

func TestDigestAsTagToShort(t *testing.T) {
	dig := digest.Digest("sha256:cc")
	_, err := digestAsTag(dig)
	expected := "too short"
	if err == nil || !strings.Contains(err.Error(), expected) {
		t.Errorf("expected error %q, actual: %v", expected, err)
	}
}

func TestDigestFromSingleStepOutput(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	input := ExampleBuildOutput1
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
	f := newDockerBuildFixture(t)
	defer f.teardown()

	input := ExamplePushOutput1
	expected := digest.Digest("sha256:cc5f4c463f81c55183d8d737ba2f0d30b3e6f3670dbe2da68f0aac168e93fbb1")
	actual, err := f.b.getDigestFromPushOutput(f.ctx, bytes.NewBuffer([]byte(input)))
	if err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

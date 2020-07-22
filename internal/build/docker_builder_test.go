package build

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockerfile"
	"github.com/tilt-dev/tilt/internal/testutils"
)

const simpleDockerfile = dockerfile.Dockerfile("FROM alpine")

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

	ref, _ := container.ParseNamedTagged("windmill.build/image:" + tag)
	if !digestMatchesRef(ref, dig) {
		t.Errorf("Expected digest %s to match ref %s", dig, ref)
	}
}

func TestDigestNotMatchesRef(t *testing.T) {
	dig := digest.Digest("sha256:cc5f4c463f81c55183d8d737ba2f0d30b3e6f3670dbe2da68f0aac168e93fbb1")
	ref, _ := container.ParseNamedTagged("windmill.build/image:tilt-deadbeef")
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

func TestDigestFromOutputV1_23(t *testing.T) {
	f := newFakeDockerBuildFixture(t)
	defer f.teardown()

	input := docker.ExampleBuildOutputV1_23
	expected := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.fakeDocker.Images["11cd0b38bc3c"] = types.ImageInspect{ID: string(expected)}
	actual, err := f.b.getDigestFromBuildOutput(f.ctx, bytes.NewBuffer([]byte(input)))
	if err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func TestDumpImageDeployRef(t *testing.T) {
	f := newFakeDockerBuildFixture(t)
	defer f.teardown()

	digest := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.fakeDocker.Images["example-image:dev"] = types.ImageInspect{ID: string(digest)}
	ref, err := f.b.DumpImageDeployRef(f.ctx, "example-image:dev")
	require.NoError(t, err)
	assert.Equal(t, "docker.io/library/example-image:tilt-11cd0eb38bc3ceb9", ref.String())
}

func makeDockerBuildErrorOutput(s string) string {
	b := &bytes.Buffer{}
	err := json.NewEncoder(b).Encode(s)
	if err != nil {
		panic(err)
	}

	return fmt.Sprintf(`{"errorDetail":{"message":%s},"error":%s}`, b.String(), b.String())
}

func TestCleanUpBuildKitErrors(t *testing.T) {
	for _, tc := range []struct {
		buildKitError     string
		expectedTiltError string
	}{
		// actual error currently emitted by buildkit when a `RUN` fails
		{
			//nolint
			buildKitError:     "failed to solve with frontend dockerfile.v0: failed to build LLB: executor failed running [/bin/sh -c go install github.com/tilt-dev/servantes/vigoda]: runc did not terminate sucessfully",
			expectedTiltError: "executor failed running [/bin/sh -c go install github.com/tilt-dev/servantes/vigoda]",
		},
		//nolint
		// artificial error - in case docker for some reason doesn't have "executor failed running", don't trim "runc did not terminate sucessfully"
		{
			//nolint
			buildKitError:     "failed to solve with frontend dockerfile.v0: failed to build LLB: [/bin/sh -c go install github.com/tilt-dev/servantes/vigoda]: runc did not terminate sucessfully",
			expectedTiltError: "[/bin/sh -c go install github.com/tilt-dev/servantes/vigoda]: runc did not terminate sucessfully",
		},
		// actual error currently emitted by buildkit when an `ADD` file is missing
		{
			buildKitError:     `failed to solve with frontend dockerfile.v0: failed to build LLB: failed to compute cache key: "/foo.txt" not found: not found`,
			expectedTiltError: `"/foo.txt" not found`,
		},
		// artificial error - in case docker fails to emit the double "not found", don't trim the one at the end
		// output in this case could do without the "failed to compute cache key", but this test is just ensuring we
		// err on the side of caution, rather than that we're emitting an optimal message for an artificial error
		{
			buildKitError:     `failed to solve with frontend dockerfile.v0: failed to build LLB: failed to compute cache key: "/foo.txt": not found`,
			expectedTiltError: `failed to compute cache key: "/foo.txt": not found`,
		},
		// artificial error - in case docker doesn't say "not found" at all
		{
			buildKitError:     `failed to solve with frontend dockerfile.v0: failed to build LLB: failed to compute cache key: "/foo.txt"`,
			expectedTiltError: `failed to compute cache key: "/foo.txt"`,
		},
		// check an unanticipated error that still has the annoying preamble
		{
			buildKitError:     "failed to solve with frontend dockerfile.v0: failed to build LLB: who knows, some made up explosion",
			expectedTiltError: "who knows, some made up explosion",
		},
		{
			// Error message when using
			// # syntax=docker/dockerfile:experimental
			//nolint
			buildKitError:     "failed to solve with frontend dockerfile.v0: failed to solve with frontend gateway.v0: rpc error: code = Unknown desc = failed to build LLB: executor failed running [/bin/sh -c pip install python-dateutil]: runc did not terminate sucessfully",
			expectedTiltError: "executor failed running [/bin/sh -c pip install python-dateutil]",
		},
	} {
		t.Run(tc.expectedTiltError, func(t *testing.T) {
			f := newFakeDockerBuildFixture(t)
			defer f.teardown()

			ctx, _, _ := testutils.CtxAndAnalyticsForTest()
			s := makeDockerBuildErrorOutput(tc.buildKitError)
			_, err := f.b.getDigestFromBuildOutput(ctx, strings.NewReader(s))
			require.NotNil(t, err)
			require.Equal(t, fmt.Sprintf("ImageBuild: %s", tc.expectedTiltError), err.Error())
		})
	}
}

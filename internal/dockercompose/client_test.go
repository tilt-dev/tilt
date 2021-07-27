package dockercompose_test

import (
	"context"
	"os"
	"testing"

	"github.com/compose-spec/compose-go/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
)

func setEnvForTest(t testing.TB, key, value string) {
	t.Helper()

	if curVal, ok := os.LookupEnv(key); ok {
		t.Cleanup(func() {
			if err := os.Setenv(key, curVal); err != nil {
				t.Errorf("Failed to restore env var %q: %v", key, err)
			}
		})
	} else {
		t.Cleanup(func() {
			if err := os.Unsetenv(key); err != nil {
				t.Errorf("Failed to restore env var %q: %v", key, err)
			}
		})
	}

	require.NoError(t, os.Setenv(key, value))
}

// TestVariableInterpolation both ensures Tilt properly passes environment to Compose for interpolation
// as well as catches potential regressions in the upstream YAML parsing from compose-go (currently, a
// fallback mechanism relies on the user having the v1 (Python) docker-compose CLI installed to work
// around bugs in the compose-go library (also used by the v2 CLI, and is susceptible to the same
// issues)
func TestVariableInterpolation(t *testing.T) {
	if testing.Short() {
		// remove this once the fallback to docker-compose CLI for YAML parse is eliminated
		// (dependent upon compose-go upstream bugs being fixed)
		t.Skip("skipping test that invokes docker-compose CLI in short mode")
	}

	f := newDCFixture(t)

	output := `services:
  app:
    command: sh -c 'node app.js'
    image: "$DC_TEST_IMAGE"
    build:
      context: $DC_TEST_CONTEXT
      dockerfile: ${DC_TEST_DOCKERFILE}
    ports:
      - target: $DC_TEST_PORT
        published: 8080
        protocol: tcp
        mode: ingress
`

	// the value is already quoted - Compose should NOT add extra quotes
	setEnvForTest(t, "DC_TEST_IMAGE", "myimage")
	// unquoted 0 is a number in YAML, but Compose SHOULD quote this properly for the field string
	// N.B. the path MUST exist or Compose will fail loading!
	setEnvForTest(t, "DC_TEST_CONTEXT", "0")
	f.tmpdir.MkdirAll("0")
	// unquoted Y is a bool in YAML, but Compose SHOULD quote this properly for the field string
	setEnvForTest(t, "DC_TEST_DOCKERFILE", "Y")
	// Compose should NOT quote this since the field is numeric
	setEnvForTest(t, "DC_TEST_PORT", "8081")

	proj := f.loadProject(output)
	if assert.Len(t, proj.Services, 1) {
		svc := proj.Services[0]
		assert.Equal(t, "myimage", svc.Image)
		if assert.NotNil(t, svc.Build) {
			assert.Equal(t, f.tmpdir.JoinPath("0"), svc.Build.Context)
			// resolved Dockerfile path is relative to the context
			assert.Equal(t, f.tmpdir.JoinPath("0", "Y"), svc.Build.Dockerfile)
		}
		if assert.Len(t, svc.Ports, 1) {
			assert.Equal(t, 8081, int(svc.Ports[0].Target))
		}
	}
}

type dcFixture struct {
	t      testing.TB
	ctx    context.Context
	cli    dockercompose.DockerComposeClient
	tmpdir *tempdir.TempDirFixture
}

func newDCFixture(t testing.TB) *dcFixture {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()

	tmpdir := tempdir.NewTempDirFixture(t)
	t.Cleanup(tmpdir.TearDown)

	return &dcFixture{
		t:      t,
		ctx:    ctx,
		cli:    dockercompose.NewDockerComposeClient(docker.LocalEnv{}),
		tmpdir: tmpdir,
	}
}

func (f *dcFixture) loadProject(composeYAML string) *types.Project {
	f.t.Helper()
	f.tmpdir.WriteFile("docker-compose.yaml", composeYAML)
	proj, err := f.cli.Project(f.ctx, []string{f.tmpdir.JoinPath("docker-compose.yaml")})
	require.NoError(f.t, err, "Failed to parse compose YAML")
	return proj
}

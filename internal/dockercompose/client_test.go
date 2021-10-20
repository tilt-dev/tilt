package dockercompose

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compose-spec/compose-go/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
)

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
	testutils.Setenv(t, "DC_TEST_IMAGE", "myimage")
	// unquoted 0 is a number in YAML, but Compose SHOULD quote this properly for the field string
	// N.B. the path MUST exist or Compose will fail loading!
	testutils.Setenv(t, "DC_TEST_CONTEXT", "0")
	f.tmpdir.MkdirAll("0")
	// unquoted Y is a bool in YAML, but Compose SHOULD quote this properly for the field string
	testutils.Setenv(t, "DC_TEST_DOCKERFILE", "Y")
	// Compose should NOT quote this since the field is numeric
	testutils.Setenv(t, "DC_TEST_PORT", "8081")

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

func TestPreferComposeV1(t *testing.T) {
	t.Run("v1 Symlink Exists", func(t *testing.T) {
		tmpdir := t.TempDir()
		v1Name := "docker-compose-v1"
		if runtime.GOOS == "windows" {
			v1Name += ".exe"
		}
		binPath := filepath.Join(tmpdir, v1Name)
		require.NoError(t, os.WriteFile(binPath, nil, 0777),
			"Failed to create fake docker-compose-v1 binary")

		testutils.Setenv(t, "PATH", tmpdir)
		cli, ok := NewDockerComposeClient(docker.LocalEnv{}).(*cmdDCClient)
		require.True(t, ok, "Unexpected type for Compose client: %T", cli)
		assert.Equal(t, binPath, cli.composePath)
	})

	t.Run("No v1 Symlink Exists", func(t *testing.T) {
		testutils.Unsetenv(t, "PATH")
		cli, ok := NewDockerComposeClient(docker.LocalEnv{}).(*cmdDCClient)
		require.True(t, ok, "Unexpected type for Compose client: %T", cli)
		// if docker-compose-v1 isn't in path, we just set the path to the unqualified binary name and let it get
		// resolved at exec time
		assert.Equal(t, "docker-compose", cli.composePath)
	})
}

func TestParseComposeVersionOutput(t *testing.T) {
	type tc struct {
		version string
		build   string
		output  []byte
	}
	tcs := []tc{
		{
			version: "v1.4.0",
			output: []byte(`docker-compose version: 1.4.0
docker-py version: 1.3.1
CPython version: 2.7.9
OpenSSL version: OpenSSL 1.0.1e 11 Feb 2013
`),
		},
		{
			version: "v1.29.2",
			build:   "5becea4c",
			output: []byte(`docker-compose version 1.29.2, build 5becea4c
docker-py version: 5.0.0
CPython version: 3.7.10
OpenSSL version: OpenSSL 1.1.0l  10 Sep 2019
`),
		},
		{
			version: "v2.0.0-rc.3",
			output:  []byte("Docker Compose version v2.0.0-rc.3\n"),
		},
		{
			version: "v2.0.0-rc.3",
			build:   "bu1ld-info",
			// NOTE: this format is valid semver but as of v2.0.0, has not been used by Compose but is supported
			output: []byte("Docker Compose version v2.0.0-rc.3+bu1ld-info\n"),
		},
	}
	for _, tc := range tcs {
		name := tc.version
		if tc.build != "" {
			name += "+" + tc.build
		}
		t.Run(name, func(t *testing.T) {
			version, build, err := parseComposeVersionOutput(tc.output)
			require.NoError(t, err)
			require.Equal(t, tc.version, version)
			require.Equal(t, tc.build, build)
		})
	}
}

func TestLoadEnvFile(t *testing.T) {
	if testing.Short() {
		// remove this once the fallback to docker-compose CLI for YAML parse is eliminated
		// (dependent upon compose-go upstream bugs being fixed)
		t.Skip("skipping test that invokes docker-compose CLI in short mode")
	}

	f := newDCFixture(t)
	f.tmpdir.WriteFile(".env", "COMMAND=foo")

	dcYAML := `services:
  foo:
    command: ${COMMAND}
    image: asdf
`
	proj := f.loadProject(dcYAML)
	require.Equal(t, types.ShellCommand{"foo"}, proj.Services[0].Command)
}

type dcFixture struct {
	t      testing.TB
	ctx    context.Context
	cli    DockerComposeClient
	tmpdir *tempdir.TempDirFixture
}

func newDCFixture(t testing.TB) *dcFixture {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()

	tmpdir := tempdir.NewTempDirFixture(t)
	t.Cleanup(tmpdir.TearDown)

	return &dcFixture{
		t:      t,
		ctx:    ctx,
		cli:    NewDockerComposeClient(docker.LocalEnv{}),
		tmpdir: tmpdir,
	}
}

func (f *dcFixture) loadProject(composeYAML string) *types.Project {
	f.t.Helper()
	f.tmpdir.WriteFile("docker-compose.yaml", composeYAML)
	_, proj, err := f.cli.Project(f.ctx, []string{f.tmpdir.JoinPath("docker-compose.yaml")})
	require.NoError(f.t, err, "Failed to parse compose YAML")
	return proj
}

package tiltfile

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestDockerignoreInSyncDir(t *testing.T) {
	f := newFixture(t)

	f.yaml("fe.yaml", deployment("fe", image("gcr.io/fe")))
	f.file("Dockerfile", `
FROM alpine
ADD ./src /src
`)
	f.file("Tiltfile", `
k8s_yaml('fe.yaml')
docker_build('gcr.io/fe', '.', live_update=[
  sync('./src', '/src')
])
`)
	f.file(".dockerignore", "build")
	f.file("Dockerfile.custom.dockerignore", "shouldntmatch")
	f.file(filepath.Join("src", "index.html"), "Hello world!")
	f.file(filepath.Join("src", ".dockerignore"), "**")

	f.load()
	m := f.assertNextManifest("fe")
	assert.Equal(t,
		[]v1alpha1.IgnoreDef{
			{
				BasePath: f.JoinPath("Tiltfile"),
			},
			{
				BasePath: f.Path(),
				Patterns: []string{"build"},
			},
			{
				BasePath: f.JoinPath("Dockerfile"),
			},
		},
		m.ImageTargetAt(0).GetFileWatchIgnores())
}

func TestNonDefaultDockerignoreInSyncDir(t *testing.T) {
	f := newFixture(t)

	f.yaml("fe.yaml", deployment("fe", image("gcr.io/fe")))
	f.file("Dockerfile.custom", `
FROM alpine
ADD ./src /src
`)
	f.file("Tiltfile", `
k8s_yaml('fe.yaml')
docker_build('gcr.io/fe', '.', dockerfile="Dockerfile.custom", live_update=[
  sync('./src', '/src')
])
`)
	f.file(".dockerignore", "shouldntmatch")
	f.file("Dockerfile.custom.dockerignore", "build")
	f.file(filepath.Join("src", "index.html"), "Hello world!")
	f.file(filepath.Join("src", "Dockerfile.custom.dockerignore"), "**")

	f.load()
	m := f.assertNextManifest("fe")
	assert.Equal(t,
		[]v1alpha1.IgnoreDef{
			{
				BasePath: f.JoinPath("Tiltfile"),
			},
			{
				BasePath: f.Path(),
				Patterns: []string{"build"},
			},
			{
				BasePath: f.JoinPath("Dockerfile.custom"),
			},
		},
		m.ImageTargetAt(0).GetFileWatchIgnores())
}

func TestCustomPlatform(t *testing.T) {
	type tc struct {
		name     string
		argValue string
		envValue string
		expected string
	}
	tcs := []tc{
		{name: "No Platform"},
		{name: "Arg Only", argValue: "linux/arm64", expected: "linux/arm64"},
		{name: "Env Only", envValue: "linux/arm64", expected: "linux/arm64"},
		// explicit arg takes precedence over env
		{name: "Arg + Env", argValue: "linux/arm64", envValue: "linux/amd64", expected: "linux/arm64"},
	}

	for _, tt := range tcs {
		t.Run(
			tt.name, func(t *testing.T) {
				if tt.envValue == "" {
					testutils.Unsetenv(t, dockerPlatformEnv)
				} else {
					testutils.Setenv(t, dockerPlatformEnv, tt.envValue)
				}

				f := newFixture(t)

				f.yaml("fe.yaml", deployment("fe", image("gcr.io/fe")))
				f.file("Dockerfile", `FROM alpine`)

				tf := "k8s_yaml('fe.yaml')\n"
				if tt.argValue == "" {
					tf += "docker_build('gcr.io/fe', '.')\n"
				} else {
					tf += fmt.Sprintf("docker_build('gcr.io/fe', '.', platform='%s')", tt.argValue)
				}

				f.file("Tiltfile", tf)

				f.load()
				m := f.assertNextManifest("fe")
				require.Equal(t, tt.expected, m.ImageTargetAt(0).DockerBuildInfo().Platform)
			})
	}
}

func TestCustomBuildDepsAreLocalRepos(t *testing.T) {
	f := newFixture(t)

	f.yaml("fe.yaml", deployment("fe", image("gcr.io/fe")))
	f.file("Dockerfile", `
FROM alpine
ADD . .
`)
	f.file("Tiltfile", `
k8s_yaml('fe.yaml')
custom_build('gcr.io/fe', 'docker build -t $EXPECTED_REF .', ['src'])
`)
	f.file(".dockerignore", "build")
	f.file(filepath.Join("src", "index.html"), "Hello world!")
	f.file(filepath.Join("src", ".git", "hi"), "hi")

	f.load()

	m := f.assertNextManifest("fe")
	it := m.ImageTargets[0]

	assert.Equal(t, []v1alpha1.IgnoreDef{
		{BasePath: f.JoinPath("Tiltfile")},
		{BasePath: f.JoinPath("src", ".git")},
	}, it.GetFileWatchIgnores())
}

func TestCustomBuildDepsZeroArgs(t *testing.T) {
	f := newFixture(t)

	f.yaml("fe.yaml", deployment("fe", image("gcr.io/fe")))
	f.file("Tiltfile", `
k8s_yaml('fe.yaml')
custom_build('gcr.io/fe', 'docker build -t $EXPECTED_REF .', [])
`)

	f.load()
}

func TestCustomBuildOutputsImageRefsTo(t *testing.T) {
	f := newFixture(t)

	f.yaml("fe.yaml", deployment("fe", image("gcr.io/fe")))
	f.file("Dockerfile", `
FROM alpine
ADD . .
`)
	f.file("Tiltfile", `
k8s_yaml('fe.yaml')
custom_build('gcr.io/fe', 'export MY_REF="gcr.io/fe:dev" && docker build -t $MY_REF . && echo $MY_REF > ref.txt',
            ['src'],
            outputs_image_ref_to='ref.txt')
`)

	f.load()

	m := f.assertNextManifest("fe")
	it := m.ImageTargets[0]
	assert.Equal(t, f.JoinPath("ref.txt"), it.CustomBuildInfo().OutputsImageRefTo)
}

func TestCustomBuildOutputsImageRefsToIncompatibleWithTag(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
custom_build('gcr.io/fe', 'export MY_REF="gcr.io/fe:dev" && docker build -t $MY_REF . && echo $MY_REF > ref.txt',
            ['src'],
            tag='dev',
            outputs_image_ref_to='ref.txt')
`)

	f.loadErrString("Cannot specify both tag= and outputs_image_ref_to=")
}

func TestExtraHosts(t *testing.T) {
	type tc struct {
		name     string
		argValue []string
		expected []string
	}
	tcs := []tc{
		{
			//docker_build('gcr.io/fe', '.')
			name: "No Extra Hosts",
		},
		{
			// docker_build('gcr.io/fe', '.', extra_hosts='testing.example.com:10.0.0.1')
			name:     "One Extra Host Only",
			argValue: []string{"testing.example.com:10.0.0.1"},
			expected: []string{"testing.example.com:10.0.0.1"},
		},
		{
			// docker_build('gcr.io/fe', '.', extra_hosts=["testing.example.com:10.0.0.1","app.testing.example.com:10.0.0.3"])
			name:     "Two Extra Hosts",
			argValue: []string{"testing.example.com:10.0.0.1", "app.testing.example.com:10.0.0.3"},
			expected: []string{"testing.example.com:10.0.0.1", "app.testing.example.com:10.0.0.3"},
		},
	}

	for _, tt := range tcs {
		t.Run(
			tt.name, func(t *testing.T) {
				f := newFixture(t)

				f.yaml("fe.yaml", deployment("fe", image("gcr.io/fe")))
				f.file("Dockerfile", `FROM alpine`)

				tf := "k8s_yaml('fe.yaml')\ndocker_build('gcr.io/fe', '.'"
				if len(tt.argValue) == 1 {
					tf += fmt.Sprintf(", extra_hosts='%s'", tt.argValue[0])
				} else if len(tt.argValue) > 1 {
					tf += `, extra_hosts=["` + strings.Join(tt.argValue, `","`) + `"]`
				}
				tf += ")"
				fmt.Println(tf)

				f.file("Tiltfile", tf)

				f.load()
				m := f.assertNextManifest("fe")
				require.Equal(t, tt.expected, m.ImageTargetAt(0).DockerBuildInfo().ExtraHosts)
			},
		)
	}
}

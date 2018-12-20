package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/container"
)

var equalitytests = []struct {
	m1       Manifest
	m2       Manifest
	expected bool
}{
	{
		Manifest{},
		Manifest{},
		true,
	},
	{
		Manifest{},
		Manifest{
			BaseDockerfile: "FROM node",
		},
		false,
	},
	{
		Manifest{
			BaseDockerfile: "FROM node",
		},
		Manifest{
			BaseDockerfile: "FROM node",
		},
		true,
	},
	{
		Manifest{
			Entrypoint: Cmd{Argv: []string{"echo", "hi"}},
		},
		Manifest{
			Entrypoint: Cmd{Argv: []string{"echo", "hi"}},
		},
		true,
	},
	{
		Manifest{
			Entrypoint: Cmd{Argv: []string{"echo", "hi"}},
		},
		Manifest{
			Entrypoint: Cmd{Argv: []string{"bash", "-c", "echo hi"}},
		},
		false,
	},
	{
		Manifest{
			repos: []LocalGithubRepo{
				LocalGithubRepo{
					LocalPath:         "/foo/baz",
					GitignoreContents: "*.exe",
				},
			},
		},
		Manifest{
			repos: []LocalGithubRepo{
				LocalGithubRepo{
					LocalPath:         "/foo/baz",
					GitignoreContents: "*.so",
				},
			},
		},
		false,
	},
	{
		Manifest{
			repos: []LocalGithubRepo{
				LocalGithubRepo{
					LocalPath:         "/foo/baz",
					GitignoreContents: "*.exe",
				},
			},
		},
		Manifest{
			repos: []LocalGithubRepo{
				LocalGithubRepo{
					LocalPath:         "/foo/baz",
					GitignoreContents: "*.exe",
				},
			},
		},
		true,
	},
	{
		Manifest{
			portForwards: []PortForward{
				{
					LocalPort: 8080,
				},
			},
		},
		Manifest{
			portForwards: []PortForward{
				{
					LocalPort: 8081,
				},
			},
		},
		false,
	},
	{
		Manifest{
			portForwards: []PortForward{
				{
					LocalPort: 8080,
				},
			},
		},
		Manifest{
			portForwards: []PortForward{
				{
					LocalPort: 8080,
				},
			},
		},
		true,
	},
	{
		Manifest{
			Steps: []Step{
				Step{
					Cmd: Cmd{Argv: []string{"bash", "-c", "hi"}},
				},
			},
		},
		Manifest{
			Steps: []Step{
				Step{
					Cmd: Cmd{Argv: []string{"bash", "-c", "hi"}},
				},
			},
		},
		true,
	},
	{
		Manifest{
			Steps: []Step{
				Step{
					Cmd: Cmd{Argv: []string{"bash", "-c", "hi"}},
				},
			},
		},
		Manifest{
			Steps: []Step{
				Step{
					Cmd: Cmd{Argv: []string{"bash", "-c", "hello"}},
				},
			},
		},
		false,
	},
	{
		Manifest{
			Steps: []Step{
				Step{
					Cmd:           Cmd{Argv: []string{"bash", "-c", "hi"}},
					Triggers:      []string{"foo"},
					BaseDirectory: "/src",
				},
			},
		},
		Manifest{
			Steps: []Step{
				Step{
					Cmd:           Cmd{Argv: []string{"bash", "-c", "hi"}},
					Triggers:      []string{"foo"},
					BaseDirectory: "/src",
				},
			},
		},
		true,
	},
	{
		Manifest{
			Steps: []Step{
				Step{
					Cmd:           Cmd{Argv: []string{"bash", "-c", "hi"}},
					Triggers:      []string{"bar"},
					BaseDirectory: "/src",
				},
			},
		},
		Manifest{
			Steps: []Step{
				Step{
					Cmd:           Cmd{Argv: []string{"bash", "-c", "hi"}},
					Triggers:      []string{"foo"},
					BaseDirectory: "/src",
				},
			},
		},
		false,
	},
	{
		Manifest{
			Steps: []Step{
				Step{
					Cmd:           Cmd{Argv: []string{"bash", "-c", "hi"}},
					Triggers:      []string{"foo"},
					BaseDirectory: "/src1",
				},
			},
		},
		Manifest{
			Steps: []Step{
				Step{
					Cmd:           Cmd{Argv: []string{"bash", "-c", "hi"}},
					Triggers:      []string{"foo"},
					BaseDirectory: "/src2",
				},
			},
		},
		false,
	},
	{
		Manifest{
			StaticBuildArgs: DockerBuildArgs{
				"foo":  "bar",
				"baz:": "qux",
			},
		},
		Manifest{
			StaticBuildArgs: DockerBuildArgs{
				"foo":  "bar",
				"baz:": "quz",
			},
		},
		false,
	},
	{
		Manifest{
			StaticBuildArgs: DockerBuildArgs{
				"foo":  "bar",
				"baz:": "qux",
			},
		},
		Manifest{
			StaticBuildArgs: DockerBuildArgs{
				"foo":  "bar",
				"baz:": "qux",
			},
		},
		true,
	},
	{
		Manifest{cachePaths: []string{"foo"}},
		Manifest{},
		false,
	},
	{
		Manifest{cachePaths: []string{"foo"}},
		Manifest{cachePaths: []string{"foo"}},
		true,
	},
	{
		Manifest{dockerignores: []Dockerignore{{"a", "b"}}},
		Manifest{dockerignores: []Dockerignore{{"b", "a"}}},
		false,
	},
	{
		Manifest{dockerignores: []Dockerignore{{"a", "b"}}},
		Manifest{dockerignores: []Dockerignore{{"a", "b"}}},
		true,
	},
	{
		Manifest{}.WithDeployInfo(DCInfo{ConfigPath: "/src/docker-compose.yml"}),
		Manifest{}.WithDeployInfo(DCInfo{ConfigPath: "/src/docker-compose.yml"}),
		true,
	},
	{
		Manifest{}.WithDeployInfo(DCInfo{ConfigPath: "/src/docker-compose1.yml"}),
		Manifest{}.WithDeployInfo(DCInfo{ConfigPath: "/src/docker-compose2.yml"}),
		false,
	},
	{
		Manifest{}.WithDeployInfo(DCInfo{YAMLRaw: []byte("hello world")}),
		Manifest{}.WithDeployInfo(DCInfo{YAMLRaw: []byte("hello world")}),
		true,
	},
	{
		Manifest{}.WithDeployInfo(DCInfo{YAMLRaw: []byte("hello world")}),
		Manifest{}.WithDeployInfo(DCInfo{YAMLRaw: []byte("goodbye world")}),
		false,
	},
	{
		Manifest{}.WithDeployInfo(DCInfo{DfRaw: []byte("hello world")}),
		Manifest{}.WithDeployInfo(DCInfo{DfRaw: []byte("hello world")}),
		true,
	},
	{
		Manifest{}.WithDeployInfo(DCInfo{DfRaw: []byte("hello world")}),
		Manifest{}.WithDeployInfo(DCInfo{DfRaw: []byte("goodbye world")}),
		false,
	},
	{
		Manifest{}.WithDeployInfo(K8sInfo{YAML: "hello world"}),
		Manifest{}.WithDeployInfo(K8sInfo{YAML: "hello world"}),
		true,
	},
	{
		Manifest{}.WithDeployInfo(K8sInfo{YAML: "hello world"}),
		Manifest{}.WithDeployInfo(K8sInfo{YAML: "goodbye world"}),
		false,
	},
}

func TestManifestEquality(t *testing.T) {
	for i, c := range equalitytests {
		actual := c.m1.Equal(c.m2)

		if actual != c.expected {
			t.Errorf("Test case #%d: Expected %+v == %+v to be %t, but got %t", i, c.m1, c.m2, c.expected, actual)
		}
	}
}

func TestManifestValidateMountRelativePath(t *testing.T) {
	mounts := []Mount{
		Mount{
			LocalPath:     "./hello",
			ContainerPath: "/src",
		},
	}
	manifest := Manifest{
		Name:           "test",
		dockerRef:      container.MustParseNamedTagged("gcr.io/some-project-162817/sancho:deadbeef"),
		BaseDockerfile: "FROM node",
		Mounts:         mounts,
	}
	err := manifest.Validate()

	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), "must be an absolute path")
	}

	manifest.Mounts[0].LocalPath = "/abs/path/hello"
	err = manifest.Validate()
	assert.Nil(t, err)

}

func TestSetPortForwards(t *testing.T) {
	m := Manifest{
		Name: "test",
	}

	m2 := m.WithPortForwards([]PortForward{
		PortForward{
			LocalPort: 8080,
		},
	})

	assert.Equal(t, 8080, m2.PortForwards()[0].LocalPort)
}

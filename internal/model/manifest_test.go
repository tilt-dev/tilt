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
			Repos: []LocalGithubRepo{
				LocalGithubRepo{
					LocalPath:         "/foo/baz",
					GitignoreContents: "*.exe",
				},
			},
		},
		Manifest{
			Repos: []LocalGithubRepo{
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
			Repos: []LocalGithubRepo{
				LocalGithubRepo{
					LocalPath:         "/foo/baz",
					GitignoreContents: "*.exe",
				},
			},
		},
		Manifest{
			Repos: []LocalGithubRepo{
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
			ConfigFiles: []string{"hi", "hello"},
		},
		Manifest{
			ConfigFiles: []string{"hi", "hello", "my"},
		},
		false,
	},
	{
		Manifest{
			ConfigFiles: []string{"my", "hi", "hello"},
		},
		Manifest{
			ConfigFiles: []string{"hi", "hello", "my"},
		},
		false,
	},
	{
		Manifest{
			ConfigFiles: []string{"hi", "hello", "my"},
		},
		Manifest{
			ConfigFiles: []string{"hi", "hello", "my"},
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
			StaticBuildArgs: map[string]string{
				"foo":  "bar",
				"baz:": "qux",
			},
		},
		Manifest{
			StaticBuildArgs: map[string]string{
				"foo":  "bar",
				"baz:": "quz",
			},
		},
		false,
	},
	{
		Manifest{
			StaticBuildArgs: map[string]string{
				"foo":  "bar",
				"baz:": "qux",
			},
		},
		Manifest{
			StaticBuildArgs: map[string]string{
				"foo":  "bar",
				"baz:": "qux",
			},
		},
		true,
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
		k8sYaml:        "yamlll",
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

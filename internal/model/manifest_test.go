package model

import (
	"testing"
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
			PortForwards: []PortForward{
				{
					LocalPort: 8080,
				},
			},
		},
		Manifest{
			PortForwards: []PortForward{
				{
					LocalPort: 8081,
				},
			},
		},
		false,
	},
	{
		Manifest{
			PortForwards: []PortForward{
				{
					LocalPort: 8080,
				},
			},
		},
		Manifest{
			PortForwards: []PortForward{
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
}

func TestManifestEquality(t *testing.T) {
	for i, c := range equalitytests {
		actual := c.m1.Equal(c.m2)

		if actual != c.expected {
			t.Errorf("Test case #%d: Expected %+v == %+v to be %t, but got %t", i, c.m1, c.m2, c.expected, actual)
		}
	}
}

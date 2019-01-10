package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/container"
)

var portFwd8000 = []PortForward{{LocalPort: 8080}}
var portFwd8001 = []PortForward{{LocalPort: 8081}}

var img1 = container.MustParseNamed("blorg.io/blorgdev/blorg-frontend:tilt-361d98a2d335373f")
var img2 = container.MustParseNamed("blorg.io/blorgdev/blorg-backend:tilt-361d98a2d335373f")

var buildArgs1 = DockerBuildArgs{
	"foo": "bar",
	"baz": "qux",
}
var buildArgs2 = DockerBuildArgs{
	"foo":  "bar",
	"beep": "boop",
}

var mount1 = Mount{
	LocalPath:     "/foo",
	ContainerPath: "/bar",
}
var mount2 = Mount{
	LocalPath:     "/baz",
	ContainerPath: "/beep",
}

var cmdSayHi = Cmd{Argv: []string{"bash", "-c", "echo hi"}}
var cmdSayBye = Cmd{Argv: []string{"bash", "-c", "echo bye"}}
var stepSayHi = Step{Cmd: cmdSayHi}
var stepSayBye = Step{Cmd: cmdSayBye}
var stepSayHiTriggerFoo = Step{
	Cmd:           cmdSayHi,
	Triggers:      []string{"foo"},
	BaseDirectory: "/src",
}
var stepSayHiTriggerBar = Step{
	Cmd:           cmdSayHi,
	Triggers:      []string{"bar"},
	BaseDirectory: "/src",
}
var stepSayHiTriggerDirA = Step{
	Cmd:           cmdSayHi,
	Triggers:      []string{"foo"},
	BaseDirectory: "/dirA",
}
var stepSayHiTriggerDirB = Step{
	Cmd:           cmdSayHi,
	Triggers:      []string{"foo"},
	BaseDirectory: "/dirB",
}

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
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{BaseDockerfile: "FROM node"}),
		},
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{BaseDockerfile: "FROM nope"}),
		},
		false,
	},
	{
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{BaseDockerfile: "FROM node"}),
		},
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{BaseDockerfile: "FROM node"}),
		},
		true,
	},
	{
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{
					Entrypoint: Cmd{Argv: []string{"echo", "hi"}},
				}),
		},
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{
					Entrypoint: Cmd{Argv: []string{"echo", "hi"}},
				}),
		},
		true,
	},
	{
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{
					Entrypoint: Cmd{Argv: []string{"echo", "hi"}},
				}),
		},
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{
					Entrypoint: Cmd{Argv: []string{"bash", "-c", "echo hi"}},
				}),
		},
		false,
	},
	{
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{Mounts: []Mount{mount1}}),
		},
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{Mounts: []Mount{mount1}}),
		},
		true,
	},
	{
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{Mounts: []Mount{mount1}}),
		},
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{Mounts: []Mount{mount2}}),
		},
		false,
	},
	{
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{Mounts: []Mount{mount1}}),
		},
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{Mounts: []Mount{mount1, mount2}}),
		},
		false,
	},
	{
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{Mounts: nil}),
		},
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{Mounts: []Mount{}}),
		},
		true,
	},
	{
		Manifest{
			ImageTarget: ImageTarget{
				repos: []LocalGitRepo{
					LocalGitRepo{
						LocalPath:         "/foo/baz",
						GitignoreContents: "*.exe",
					},
				},
			},
		},
		Manifest{
			ImageTarget: ImageTarget{
				repos: []LocalGitRepo{
					LocalGitRepo{
						LocalPath:         "/foo/baz",
						GitignoreContents: "*.so",
					},
				},
			},
		},
		false,
	},
	{
		Manifest{
			ImageTarget: ImageTarget{
				repos: []LocalGitRepo{
					LocalGitRepo{
						LocalPath:         "/foo/baz",
						GitignoreContents: "*.exe",
					},
				},
			},
		},
		Manifest{
			ImageTarget: ImageTarget{
				repos: []LocalGitRepo{
					LocalGitRepo{
						LocalPath:         "/foo/baz",
						GitignoreContents: "*.exe",
					},
				},
			},
		},
		true,
	},
	{
		Manifest{}.WithDeployTarget(K8sTarget{PortForwards: portFwd8000}),
		Manifest{}.WithDeployTarget(K8sTarget{PortForwards: portFwd8001}),
		false,
	},
	{
		Manifest{}.WithDeployTarget(K8sTarget{PortForwards: portFwd8000}),
		Manifest{}.WithDeployTarget(K8sTarget{PortForwards: portFwd8000}),
		true,
	},
	{
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{Steps: []Step{stepSayHi}},
			),
		},
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{Steps: []Step{stepSayHi}},
			),
		},
		true,
	},
	{
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{Steps: []Step{stepSayHi}},
			),
		},
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{Steps: []Step{stepSayBye}},
			),
		},
		false,
	},
	{
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{Steps: []Step{stepSayHiTriggerFoo}},
			),
		},
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{Steps: []Step{stepSayHiTriggerFoo}},
			),
		},
		true,
	},
	{
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{Steps: []Step{stepSayHiTriggerFoo}},
			),
		},
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{Steps: []Step{stepSayHiTriggerBar}}),
		},
		false,
	},
	{
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{Steps: []Step{stepSayHiTriggerDirA}},
			),
		},
		Manifest{
			ImageTarget: ImageTarget{}.WithBuildDetails(
				FastBuild{Steps: []Step{stepSayHiTriggerDirB}},
			),
		},
		false,
	},
	{
		Manifest{ImageTarget: ImageTarget{}.WithBuildDetails(StaticBuild{Dockerfile: "FROM foo"})},
		Manifest{ImageTarget: ImageTarget{}.WithBuildDetails(StaticBuild{Dockerfile: "FROM bar"})},
		false,
	},
	{
		Manifest{ImageTarget: ImageTarget{}.WithBuildDetails(StaticBuild{Dockerfile: "FROM foo"})},
		Manifest{ImageTarget: ImageTarget{}.WithBuildDetails(StaticBuild{Dockerfile: "FROM foo"})},
		true,
	},
	{
		Manifest{ImageTarget: ImageTarget{}.WithBuildDetails(StaticBuild{BuildPath: "foo/bar"})},
		Manifest{ImageTarget: ImageTarget{}.WithBuildDetails(StaticBuild{BuildPath: "foo/bar/baz"})},
		false,
	},
	{
		Manifest{ImageTarget: ImageTarget{}.WithBuildDetails(StaticBuild{BuildPath: "foo/bar"})},
		Manifest{ImageTarget: ImageTarget{}.WithBuildDetails(StaticBuild{BuildPath: "foo/bar"})},
		true,
	},
	{
		Manifest{ImageTarget: ImageTarget{}.WithBuildDetails(StaticBuild{BuildArgs: buildArgs1})},
		Manifest{ImageTarget: ImageTarget{}.WithBuildDetails(StaticBuild{BuildArgs: buildArgs2})},
		false,
	},
	{
		Manifest{ImageTarget: ImageTarget{}.WithBuildDetails(StaticBuild{BuildArgs: buildArgs1})},
		Manifest{ImageTarget: ImageTarget{}.WithBuildDetails(StaticBuild{BuildArgs: buildArgs1})},
		true,
	},
	{
		Manifest{ImageTarget: ImageTarget{cachePaths: []string{"foo"}}},
		Manifest{ImageTarget: ImageTarget{cachePaths: []string{"bar"}}},
		false,
	},
	{
		Manifest{ImageTarget: ImageTarget{cachePaths: []string{"foo"}}},
		Manifest{ImageTarget: ImageTarget{cachePaths: []string{"foo"}}},
		true,
	},
	{
		Manifest{ImageTarget: ImageTarget{Ref: img1}},
		Manifest{ImageTarget: ImageTarget{Ref: img2}},
		false,
	},
	{
		Manifest{ImageTarget: ImageTarget{Ref: img1}},
		Manifest{ImageTarget: ImageTarget{Ref: img1}},
		true,
	},
	{
		Manifest{ImageTarget: ImageTarget{dockerignores: []Dockerignore{{"a", "b"}}}},
		Manifest{ImageTarget: ImageTarget{dockerignores: []Dockerignore{{"b", "a"}}}},
		false,
	},
	{
		Manifest{ImageTarget: ImageTarget{dockerignores: []Dockerignore{{"a", "b"}}}},
		Manifest{ImageTarget: ImageTarget{dockerignores: []Dockerignore{{"a", "b"}}}},
		true,
	},
	{
		Manifest{}.WithDeployTarget(DockerComposeTarget{ConfigPath: "/src/docker-compose.yml"}),
		Manifest{}.WithDeployTarget(DockerComposeTarget{ConfigPath: "/src/docker-compose.yml"}),
		true,
	},
	{
		Manifest{}.WithDeployTarget(DockerComposeTarget{ConfigPath: "/src/docker-compose1.yml"}),
		Manifest{}.WithDeployTarget(DockerComposeTarget{ConfigPath: "/src/docker-compose2.yml"}),
		false,
	},
	{
		Manifest{}.WithDeployTarget(DockerComposeTarget{YAMLRaw: []byte("hello world")}),
		Manifest{}.WithDeployTarget(DockerComposeTarget{YAMLRaw: []byte("hello world")}),
		true,
	},
	{
		Manifest{}.WithDeployTarget(DockerComposeTarget{YAMLRaw: []byte("hello world")}),
		Manifest{}.WithDeployTarget(DockerComposeTarget{YAMLRaw: []byte("goodbye world")}),
		false,
	},
	{
		Manifest{}.WithDeployTarget(DockerComposeTarget{DfRaw: []byte("hello world")}),
		Manifest{}.WithDeployTarget(DockerComposeTarget{DfRaw: []byte("hello world")}),
		true,
	},
	{
		Manifest{}.WithDeployTarget(DockerComposeTarget{DfRaw: []byte("hello world")}),
		Manifest{}.WithDeployTarget(DockerComposeTarget{DfRaw: []byte("goodbye world")}),
		false,
	},
	{
		Manifest{}.WithDeployTarget(K8sTarget{YAML: "hello world"}),
		Manifest{}.WithDeployTarget(K8sTarget{YAML: "hello world"}),
		true,
	},
	{
		Manifest{}.WithDeployTarget(K8sTarget{YAML: "hello world"}),
		Manifest{}.WithDeployTarget(K8sTarget{YAML: "goodbye world"}),
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
	fbInfo := FastBuild{
		BaseDockerfile: `FROM golang`,
		Mounts: []Mount{
			Mount{
				LocalPath:     "./hello",
				ContainerPath: "/src",
			},
		},
	}

	manifest := Manifest{
		Name:        "test",
		ImageTarget: ImageTarget{Ref: img1}.WithBuildDetails(fbInfo),
	}
	err := manifest.Validate()

	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), "must be an absolute path")
	}

	fbInfo.Mounts[0].LocalPath = "/abs/path/hello"
	manifest.ImageTarget = ImageTarget{Ref: img1}.WithBuildDetails(fbInfo)
	err = manifest.Validate()
	assert.Nil(t, err)

}

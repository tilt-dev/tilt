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
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{BaseDockerfile: "FROM node"}),
		},
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{BaseDockerfile: "FROM nope"}),
		},
		false,
	},
	{
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{BaseDockerfile: "FROM node"}),
		},
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{BaseDockerfile: "FROM node"}),
		},
		true,
	},
	{
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{
					Entrypoint: Cmd{Argv: []string{"echo", "hi"}},
				}),
		},
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{
					Entrypoint: Cmd{Argv: []string{"echo", "hi"}},
				}),
		},
		true,
	},
	{
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{
					Entrypoint: Cmd{Argv: []string{"echo", "hi"}},
				}),
		},
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{
					Entrypoint: Cmd{Argv: []string{"bash", "-c", "echo hi"}},
				}),
		},
		false,
	},
	{
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{Mounts: []Mount{mount1}}),
		},
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{Mounts: []Mount{mount1}}),
		},
		true,
	},
	{
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{Mounts: []Mount{mount1}}),
		},
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{Mounts: []Mount{mount2}}),
		},
		false,
	},
	{
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{Mounts: []Mount{mount1}}),
		},
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{Mounts: []Mount{mount1, mount2}}),
		},
		false,
	},
	{
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{Mounts: nil}),
		},
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{Mounts: []Mount{}}),
		},
		true,
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
		Manifest{}.WithDeployInfo(K8sInfo{PortForwards: portFwd8000}),
		Manifest{}.WithDeployInfo(K8sInfo{PortForwards: portFwd8001}),
		false,
	},
	{
		Manifest{}.WithDeployInfo(K8sInfo{PortForwards: portFwd8000}),
		Manifest{}.WithDeployInfo(K8sInfo{PortForwards: portFwd8000}),
		true,
	},
	{
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{Steps: []Step{stepSayHi}},
			),
		},
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{Steps: []Step{stepSayHi}},
			),
		},
		true,
	},
	{
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{Steps: []Step{stepSayHi}},
			),
		},
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{Steps: []Step{stepSayBye}},
			),
		},
		false,
	},
	{
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{Steps: []Step{stepSayHiTriggerFoo}},
			),
		},
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{Steps: []Step{stepSayHiTriggerFoo}},
			),
		},
		true,
	},
	{
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{Steps: []Step{stepSayHiTriggerFoo}},
			),
		},
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{Steps: []Step{stepSayHiTriggerBar}}),
		},
		false,
	},
	{
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{Steps: []Step{stepSayHiTriggerDirA}},
			),
		},
		Manifest{
			DockerInfo: DockerInfo{}.WithBuildDetails(
				FastBuild{Steps: []Step{stepSayHiTriggerDirB}},
			),
		},
		false,
	},
	{
		Manifest{DockerInfo: DockerInfo{}.WithBuildDetails(StaticBuild{Dockerfile: "FROM foo"})},
		Manifest{DockerInfo: DockerInfo{}.WithBuildDetails(StaticBuild{Dockerfile: "FROM bar"})},
		false,
	},
	{
		Manifest{DockerInfo: DockerInfo{}.WithBuildDetails(StaticBuild{Dockerfile: "FROM foo"})},
		Manifest{DockerInfo: DockerInfo{}.WithBuildDetails(StaticBuild{Dockerfile: "FROM foo"})},
		true,
	},
	{
		Manifest{DockerInfo: DockerInfo{}.WithBuildDetails(StaticBuild{BuildPath: "foo/bar"})},
		Manifest{DockerInfo: DockerInfo{}.WithBuildDetails(StaticBuild{BuildPath: "foo/bar/baz"})},
		false,
	},
	{
		Manifest{DockerInfo: DockerInfo{}.WithBuildDetails(StaticBuild{BuildPath: "foo/bar"})},
		Manifest{DockerInfo: DockerInfo{}.WithBuildDetails(StaticBuild{BuildPath: "foo/bar"})},
		true,
	},
	{
		Manifest{DockerInfo: DockerInfo{}.WithBuildDetails(StaticBuild{BuildArgs: buildArgs1})},
		Manifest{DockerInfo: DockerInfo{}.WithBuildDetails(StaticBuild{BuildArgs: buildArgs2})},
		false,
	},
	{
		Manifest{DockerInfo: DockerInfo{}.WithBuildDetails(StaticBuild{BuildArgs: buildArgs1})},
		Manifest{DockerInfo: DockerInfo{}.WithBuildDetails(StaticBuild{BuildArgs: buildArgs1})},
		true,
	},
	{
		Manifest{DockerInfo: DockerInfo{cachePaths: []string{"foo"}}},
		Manifest{DockerInfo: DockerInfo{cachePaths: []string{"bar"}}},
		false,
	},
	{
		Manifest{DockerInfo: DockerInfo{cachePaths: []string{"foo"}}},
		Manifest{DockerInfo: DockerInfo{cachePaths: []string{"foo"}}},
		true,
	},
	{
		Manifest{DockerInfo: DockerInfo{DockerRef: img1}},
		Manifest{DockerInfo: DockerInfo{DockerRef: img2}},
		false,
	},
	{
		Manifest{DockerInfo: DockerInfo{DockerRef: img1}},
		Manifest{DockerInfo: DockerInfo{DockerRef: img1}},
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
	fbInfo := FastBuild{
		Mounts: []Mount{
			Mount{
				LocalPath:     "./hello",
				ContainerPath: "/src",
			},
		},
	}

	manifest := Manifest{
		Name:       "test",
		DockerInfo: DockerInfo{}.WithBuildDetails(fbInfo),
	}
	err := manifest.Validate()

	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), "must be an absolute path")
	}

	fbInfo.Mounts[0].LocalPath = "/abs/path/hello"
	manifest.DockerInfo = DockerInfo{}.WithBuildDetails(fbInfo)
	err = manifest.Validate()
	assert.Nil(t, err)

}

package model

import (
	"testing"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/container"
)

var portFwd8000 = []PortForward{{LocalPort: 8080}}
var portFwd8001 = []PortForward{{LocalPort: 8081}}

var img1 = container.MustParseSelector("blorg.io/blorgdev/blorg-frontend:tilt-361d98a2d335373f")
var img2 = container.MustParseSelector("blorg.io/blorgdev/blorg-backend:tilt-361d98a2d335373f")

var buildArgs1 = DockerBuildArgs{
	"foo": "bar",
	"baz": "qux",
}
var buildArgs2 = DockerBuildArgs{
	"foo":  "bar",
	"beep": "boop",
}

var sync1 = Sync{
	LocalPath:     "/foo",
	ContainerPath: "/bar",
}
var sync2 = Sync{
	LocalPath:     "/baz",
	ContainerPath: "/beep",
}

var cmdSayHi = Cmd{Argv: []string{"bash", "-c", "echo hi"}}
var cmdSayBye = Cmd{Argv: []string{"bash", "-c", "echo bye"}}
var stepSayHi = Run{Cmd: cmdSayHi}
var stepSayBye = Run{Cmd: cmdSayBye}
var stepSayHiTriggerFoo = Run{
	Cmd:      cmdSayHi,
	Triggers: NewPathSet([]string{"foo"}, "/src"),
}
var stepSayHiTriggerBar = Run{
	Cmd:      cmdSayHi,
	Triggers: NewPathSet([]string{"bar"}, "/src"),
}
var stepSayHiTriggerDirA = Run{
	Cmd:      cmdSayHi,
	Triggers: NewPathSet([]string{"foo"}, "/dirA"),
}
var stepSayHiTriggerDirB = Run{
	Cmd:      cmdSayHi,
	Triggers: NewPathSet([]string{"foo"}, "/dirB"),
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
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{BaseDockerfile: "FROM node"})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{BaseDockerfile: "FROM nope"})),
		false,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{BaseDockerfile: "FROM node"})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{BaseDockerfile: "FROM node"})),
		true,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{
				Entrypoint: Cmd{Argv: []string{"echo", "hi"}},
			})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{
				Entrypoint: Cmd{Argv: []string{"echo", "hi"}},
			})),
		true,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{
				Entrypoint: Cmd{Argv: []string{"echo", "hi"}},
			})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{
				Entrypoint: Cmd{Argv: []string{"bash", "-c", "echo hi"}},
			})),
		false,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{Syncs: []Sync{sync1}})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{Syncs: []Sync{sync1}})),
		true,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{Syncs: []Sync{sync1}})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{Syncs: []Sync{sync2}})),
		false,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{Syncs: []Sync{sync1}})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{Syncs: []Sync{sync1, sync2}})),
		false,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{Syncs: nil})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{Syncs: []Sync{}})),
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
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{Runs: []Run{stepSayHi}},
		)),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{Runs: []Run{stepSayHi}},
		)),
		true,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{Runs: []Run{stepSayHi}},
		)),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{Runs: []Run{stepSayBye}},
		)),
		false,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{Runs: []Run{stepSayHiTriggerFoo}},
		)),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{Runs: []Run{stepSayHiTriggerFoo}},
		)),
		true,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{Runs: []Run{stepSayHiTriggerFoo}},
		)),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{Runs: []Run{stepSayHiTriggerBar}})),
		false,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{Runs: []Run{stepSayHiTriggerDirA}},
		)),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(
			FastBuild{Runs: []Run{stepSayHiTriggerDirB}},
		)),
		false,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{Dockerfile: "FROM foo"})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{Dockerfile: "FROM bar"})),
		false,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{Dockerfile: "FROM foo"})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{Dockerfile: "FROM foo"})),
		true,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildPath: "foo/bar"})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildPath: "foo/bar/baz"})),
		false,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildPath: "foo/bar"})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildPath: "foo/bar"})),
		true,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildArgs: buildArgs1})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildArgs: buildArgs2})),
		false,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildArgs: buildArgs1})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildArgs: buildArgs1})),
		true,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{cachePaths: []string{"foo"}}),
		Manifest{}.WithImageTarget(ImageTarget{cachePaths: []string{"bar"}}),
		false,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{cachePaths: []string{"foo"}}),
		Manifest{}.WithImageTarget(ImageTarget{cachePaths: []string{"foo"}}),
		true,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{ConfigurationRef: img1}),
		Manifest{}.WithImageTarget(ImageTarget{ConfigurationRef: img2}),
		false,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{ConfigurationRef: img1}),
		Manifest{}.WithImageTarget(ImageTarget{ConfigurationRef: img1}),
		true,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{DeploymentRef: img1.AsNamedOnly()}),
		Manifest{}.WithImageTarget(ImageTarget{DeploymentRef: img2.AsNamedOnly()}),
		false,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{DeploymentRef: img1.AsNamedOnly()}),
		Manifest{}.WithImageTarget(ImageTarget{DeploymentRef: img1.AsNamedOnly()}),
		true,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{dockerignores: []Dockerignore{{"a", "b"}}}),
		Manifest{}.WithImageTarget(ImageTarget{dockerignores: []Dockerignore{{"b", "a"}}}),
		false,
	},
	{
		Manifest{}.WithImageTarget(ImageTarget{dockerignores: []Dockerignore{{"a", "b"}}}),
		Manifest{}.WithImageTarget(ImageTarget{dockerignores: []Dockerignore{{"a", "b"}}}),
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
	{
		Manifest{}.WithDeployTarget(K8sTarget{
			ExtraPodSelectors: []labels.Selector{labels.Set{"foo": "bar"}.AsSelector()},
		}),
		Manifest{}.WithDeployTarget(K8sTarget{
			ExtraPodSelectors: []labels.Selector{labels.Set{"foo": "bar"}.AsSelector()},
		}),
		true,
	},
	{
		Manifest{}.WithDeployTarget(K8sTarget{
			ExtraPodSelectors: []labels.Selector{labels.Set{"foo": "bar"}.AsSelector()},
		}),
		Manifest{}.WithDeployTarget(K8sTarget{
			ExtraPodSelectors: []labels.Selector{labels.Set{"foo": "baz"}.AsSelector()},
		}),
		false,
	},
	{
		Manifest{TriggerMode: TriggerModeManual},
		Manifest{TriggerMode: TriggerModeManual},
		true,
	},
	{
		Manifest{TriggerMode: TriggerModeAuto},
		Manifest{TriggerMode: TriggerModeManual},
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

func TestManifestValidateSyncRelativePath(t *testing.T) {
	fbInfo := FastBuild{
		BaseDockerfile: `FROM golang`,
		Syncs: []Sync{
			Sync{
				LocalPath:     "./hello",
				ContainerPath: "/src",
			},
		},
	}

	manifest := Manifest{
		Name: "test",
	}.WithImageTarget(ImageTarget{ConfigurationRef: img1}.WithBuildDetails(fbInfo))
	err := manifest.Validate()

	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), "must be an absolute path")
	}

	fbInfo.Syncs[0].LocalPath = "/abs/path/hello"
	manifest = manifest.WithImageTarget(ImageTarget{ConfigurationRef: img1}.WithBuildDetails(fbInfo))
	err = manifest.Validate()
	assert.Nil(t, err)
}

func TestDCTargetValidate(t *testing.T) {
	targ := DockerComposeTarget{
		Name:       "blah",
		ConfigPath: "docker-compose.yml",
	}
	err := targ.Validate()
	assert.NoError(t, err)

	noConfPath := DockerComposeTarget{Name: "blah"}
	err = noConfPath.Validate()
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "missing config path")
	}

	noName := DockerComposeTarget{ConfigPath: "docker-compose.yml"}
	err = noName.Validate()
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "missing name")
	}
}

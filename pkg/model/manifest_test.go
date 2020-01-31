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
	name                string
	m1                  Manifest
	m2                  Manifest
	expectedEqual       bool
	expectedInvalidates bool
}{
	{
		"empty manifests equal",
		Manifest{},
		Manifest{},
		true,
		false,
	},
	{
		"PortForwards unequal",
		Manifest{}.WithDeployTarget(K8sTarget{PortForwards: portFwd8000}),
		Manifest{}.WithDeployTarget(K8sTarget{PortForwards: portFwd8001}),
		false,
		true,
	},
	{
		"PortForwards equal",
		Manifest{}.WithDeployTarget(K8sTarget{PortForwards: portFwd8000}),
		Manifest{}.WithDeployTarget(K8sTarget{PortForwards: portFwd8000}),
		true,
		false,
	},
	{
		"DockerBuild.Dockerfile unequal",
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{Dockerfile: "FROM foo"})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{Dockerfile: "FROM bar"})),
		false,
		true,
	},
	{
		"DockerBuild.Dockerfile equal",
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{Dockerfile: "FROM foo"})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{Dockerfile: "FROM foo"})),
		true,
		false,
	},
	{
		"DockerBuild.BuildPath unequal",
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildPath: "foo/bar"})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildPath: "foo/bar/baz"})),
		false,
		true,
	},
	{
		"DockerBuild.BuildPath equal",
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildPath: "foo/bar"})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildPath: "foo/bar"})),
		true,
		false,
	},
	{
		"DockerBuild.BuildArgs unequal",
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildArgs: buildArgs1})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildArgs: buildArgs2})),
		false,
		true,
	},
	{
		"DockerBuild.BuildArgs equal",
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildArgs: buildArgs1})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildArgs: buildArgs1})),
		true,
		false,
	},
	{
		"ImageTarget.cachePaths unequal",
		Manifest{}.WithImageTarget(ImageTarget{cachePaths: []string{"foo"}}),
		Manifest{}.WithImageTarget(ImageTarget{cachePaths: []string{"bar"}}),
		false,
		true,
	},
	{
		"ImageTarget.cachePaths equal",
		Manifest{}.WithImageTarget(ImageTarget{cachePaths: []string{"foo"}}),
		Manifest{}.WithImageTarget(ImageTarget{cachePaths: []string{"foo"}}),
		true,
		false,
	},
	{
		"ImageTarget.ConfigurationRef unequal",
		Manifest{}.WithImageTarget(ImageTarget{Refs: container.RefSet{ConfigurationRef: img1}}),
		Manifest{}.WithImageTarget(ImageTarget{Refs: container.RefSet{ConfigurationRef: img2}}),
		false,
		true,
	},
	{
		"ImageTarget.ConfigurationRef equal",
		Manifest{}.WithImageTarget(ImageTarget{Refs: container.RefSet{ConfigurationRef: img1}}),
		Manifest{}.WithImageTarget(ImageTarget{Refs: container.RefSet{ConfigurationRef: img1}}),
		true,
		false,
	},
	{
		"ImageTarget.DockerIgnores unequal",
		Manifest{}.WithImageTarget(ImageTarget{dockerignores: []Dockerignore{{"a", "b"}}}),
		Manifest{}.WithImageTarget(ImageTarget{dockerignores: []Dockerignore{{"b", "a"}}}),
		false,
		true,
	},
	{
		"ImageTarget.DockerIgnores equal",
		Manifest{}.WithImageTarget(ImageTarget{dockerignores: []Dockerignore{{"a", "b"}}}),
		Manifest{}.WithImageTarget(ImageTarget{dockerignores: []Dockerignore{{"a", "b"}}}),
		true,
		false,
	},
	{
		"DockerCompose.ConfigPaths equal",
		Manifest{}.WithDeployTarget(DockerComposeTarget{ConfigPaths: []string{"/src/docker-compose.yml"}}),
		Manifest{}.WithDeployTarget(DockerComposeTarget{ConfigPaths: []string{"/src/docker-compose.yml"}}),
		true,
		false,
	},
	{
		"DockerCompose.ConfigPaths unequal",
		Manifest{}.WithDeployTarget(DockerComposeTarget{ConfigPaths: []string{"/src/docker-compose1.yml"}}),
		Manifest{}.WithDeployTarget(DockerComposeTarget{ConfigPaths: []string{"/src/docker-compose2.yml"}}),
		false,
		true,
	},
	{
		"DockerCompose.YAMLRaw equal",
		Manifest{}.WithDeployTarget(DockerComposeTarget{YAMLRaw: []byte("hello world")}),
		Manifest{}.WithDeployTarget(DockerComposeTarget{YAMLRaw: []byte("hello world")}),
		true,
		false,
	},
	{
		"DockerCompose.YAMLRaw unequal",
		Manifest{}.WithDeployTarget(DockerComposeTarget{YAMLRaw: []byte("hello world")}),
		Manifest{}.WithDeployTarget(DockerComposeTarget{YAMLRaw: []byte("goodbye world")}),
		false,
		true,
	},
	{
		"DockerCompose.DfRaw equal",
		Manifest{}.WithDeployTarget(DockerComposeTarget{DfRaw: []byte("hello world")}),
		Manifest{}.WithDeployTarget(DockerComposeTarget{DfRaw: []byte("hello world")}),
		true,
		false,
	},
	{
		"DockerCompose.DfRaw unequal",
		Manifest{}.WithDeployTarget(DockerComposeTarget{DfRaw: []byte("hello world")}),
		Manifest{}.WithDeployTarget(DockerComposeTarget{DfRaw: []byte("goodbye world")}),
		false,
		true,
	},
	{
		"k8s.YAML equal",
		Manifest{}.WithDeployTarget(K8sTarget{YAML: "hello world"}),
		Manifest{}.WithDeployTarget(K8sTarget{YAML: "hello world"}),
		true,
		false,
	},
	{
		"k8s.YAML unequal",
		Manifest{}.WithDeployTarget(K8sTarget{YAML: "hello world"}),
		Manifest{}.WithDeployTarget(K8sTarget{YAML: "goodbye world"}),
		false,
		true,
	},
	{
		"k8s.ExtraPodSelectors equal",
		Manifest{}.WithDeployTarget(K8sTarget{
			ExtraPodSelectors: []labels.Selector{labels.Set{"foo": "bar"}.AsSelector()},
		}),
		Manifest{}.WithDeployTarget(K8sTarget{
			ExtraPodSelectors: []labels.Selector{labels.Set{"foo": "bar"}.AsSelector()},
		}),
		true,
		false,
	},
	{
		"k8s.ExtraPodSelectors unequal",
		Manifest{}.WithDeployTarget(K8sTarget{
			ExtraPodSelectors: []labels.Selector{labels.Set{"foo": "bar"}.AsSelector()},
		}),
		Manifest{}.WithDeployTarget(K8sTarget{
			ExtraPodSelectors: []labels.Selector{labels.Set{"foo": "baz"}.AsSelector()},
		}),
		false,
		true,
	},
	{
		"TriggerMode equal",
		Manifest{TriggerMode: TriggerModeManualAfterInitial},
		Manifest{TriggerMode: TriggerModeManualAfterInitial},
		true,
		false,
	},
	{
		"TriggerMode unequal",
		Manifest{TriggerMode: TriggerModeAuto},
		Manifest{TriggerMode: TriggerModeManualAfterInitial},
		false,
		false,
	},
	{
		"Name equal",
		Manifest{Name: "foo"},
		Manifest{Name: "bar"},
		false,
		false,
	},
	{
		"Name & k8s YAML unequal",
		Manifest{Name: "foo"}.WithDeployTarget(K8sTarget{YAML: "hello world"}),
		Manifest{Name: "bar"}.WithDeployTarget(K8sTarget{YAML: "goodbye world"}),
		false,
		true,
	},
	{
		"LocalTarget equal",
		Manifest{}.WithDeployTarget(NewLocalTarget("foo", ToShellCmd("beep boop"), Cmd{}, []string{"bar", "baz"}, "path/to/tiltfile")),
		Manifest{}.WithDeployTarget(NewLocalTarget("foo", ToShellCmd("beep boop"), Cmd{}, []string{"bar", "baz"}, "path/to/tiltfile")),
		true,
		false,
	},
	{
		"LocalTarget.Name unequal",
		Manifest{}.WithDeployTarget(NewLocalTarget("foo", ToShellCmd("beep boop"), Cmd{}, []string{"bar", "baz"}, "path/to/tiltfile")),
		Manifest{}.WithDeployTarget(NewLocalTarget("foooooo", ToShellCmd("beep boop"), Cmd{}, []string{"bar", "baz"}, "path/to/tiltfile")),
		false,
		true,
	},
	{
		"LocalTarget.UpdateCmd unequal",
		Manifest{}.WithDeployTarget(NewLocalTarget("foo", ToShellCmd("beep boop"), Cmd{}, []string{"bar", "baz"}, "path/to/tiltfile")),
		Manifest{}.WithDeployTarget(NewLocalTarget("foo", ToShellCmd("bippity boppity"), Cmd{}, []string{"bar", "baz"}, "path/to/tiltfile")),
		false,
		true,
	},
	{
		"LocalTarget.Deps unequal",
		Manifest{}.WithDeployTarget(NewLocalTarget("foo", ToShellCmd("beep boop"), Cmd{}, []string{"bar", "baz"}, "path/to/tiltfile")),
		Manifest{}.WithDeployTarget(NewLocalTarget("foo", ToShellCmd("beep boop"), Cmd{}, []string{"quux", "baz"}, "path/to/tiltfile")),
		false,
		true,
	},
	{
		"LocalTarget.workdir unequal",
		Manifest{}.WithDeployTarget(NewLocalTarget("foo", ToShellCmd("beep boop"), Cmd{}, []string{"bar", "baz"}, "path/to/tiltfile")),
		Manifest{}.WithDeployTarget(NewLocalTarget("foo", ToShellCmd("beep boop"), Cmd{}, []string{"bar", "baz"}, "some/other/path")),
		false,
		true,
	},
}

func TestManifestEquality(t *testing.T) {
	for i, c := range equalitytests {
		actualEqual := c.m1.Equal(c.m2)

		if actualEqual != c.expectedEqual {
			t.Errorf("Test case %s (#%d): Expected %+v == %+v to be %t, but got %t", c.name, i, c.m1, c.m2, c.expectedEqual, actualEqual)
		}

		actualInvalidates := ChangesInvalidateBuild(c.m1, c.m2)

		if actualInvalidates != c.expectedInvalidates {
			t.Errorf("Test case %s (#%d): Expected %+v => %+v InvalidatesBuild = %t, but got %t", c.name, i, c.m1, c.m2, c.expectedInvalidates, actualInvalidates)
		}
	}
}

func TestDCTargetValidate(t *testing.T) {
	targ := DockerComposeTarget{
		Name:        "blah",
		ConfigPaths: []string{"docker-compose.yml"},
	}
	err := targ.Validate()
	assert.NoError(t, err)

	noConfPath := DockerComposeTarget{Name: "blah"}
	err = noConfPath.Validate()
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "missing config path")
	}

	noName := DockerComposeTarget{ConfigPaths: []string{"docker-compose.yml"}}
	err = noName.Validate()
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "missing name")
	}
}

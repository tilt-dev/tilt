package model

import (
	"testing"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/container"
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

var equalitytests = []struct {
	name                string
	m1                  Manifest
	m2                  Manifest
	expectedInvalidates bool
}{
	{
		"empty manifests equal",
		Manifest{},
		Manifest{},
		false,
	},
	{
		"PortForwards unequal",
		Manifest{}.WithDeployTarget(K8sTarget{PortForwards: portFwd8000}),
		Manifest{}.WithDeployTarget(K8sTarget{PortForwards: portFwd8001}),
		true,
	},
	{
		"PortForwards equal",
		Manifest{}.WithDeployTarget(K8sTarget{PortForwards: portFwd8000}),
		Manifest{}.WithDeployTarget(K8sTarget{PortForwards: portFwd8000}),
		false,
	},
	{
		"DockerBuild.Dockerfile unequal",
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{Dockerfile: "FROM foo"})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{Dockerfile: "FROM bar"})),
		true,
	},
	{
		"DockerBuild.Dockerfile equal",
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{Dockerfile: "FROM foo"})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{Dockerfile: "FROM foo"})),
		false,
	},
	{
		"DockerBuild.BuildPath unequal",
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildPath: "foo/bar"})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildPath: "foo/bar/baz"})),
		true,
	},
	{
		"DockerBuild.BuildPath equal",
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildPath: "foo/bar"})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildPath: "foo/bar"})),
		false,
	},
	{
		"DockerBuild.BuildArgs unequal",
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildArgs: buildArgs1})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildArgs: buildArgs2})),
		true,
	},
	{
		"DockerBuild.BuildArgs equal",
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildArgs: buildArgs1})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{BuildArgs: buildArgs1})),
		false,
	},
	{
		"ImageTarget.cachePaths unequal",
		Manifest{}.WithImageTarget(ImageTarget{cachePaths: []string{"foo"}}),
		Manifest{}.WithImageTarget(ImageTarget{cachePaths: []string{"bar"}}),
		true,
	},
	{
		"ImageTarget.cachePaths equal",
		Manifest{}.WithImageTarget(ImageTarget{cachePaths: []string{"foo"}}),
		Manifest{}.WithImageTarget(ImageTarget{cachePaths: []string{"foo"}}),
		false,
	},
	{
		"ImageTarget.ConfigurationRef unequal",
		Manifest{}.WithImageTarget(ImageTarget{Refs: container.RefSet{ConfigurationRef: img1}}),
		Manifest{}.WithImageTarget(ImageTarget{Refs: container.RefSet{ConfigurationRef: img2}}),
		true,
	},
	{
		"ImageTarget.ConfigurationRef equal",
		Manifest{}.WithImageTarget(ImageTarget{Refs: container.RefSet{ConfigurationRef: img1}}),
		Manifest{}.WithImageTarget(ImageTarget{Refs: container.RefSet{ConfigurationRef: img1}}),
		false,
	},
	{
		"ImageTarget.DockerIgnores unequal",
		Manifest{}.WithImageTarget(ImageTarget{dockerignores: []Dockerignore{{LocalPath: "a", Patterns: []string{"b"}}}}),
		Manifest{}.WithImageTarget(ImageTarget{dockerignores: []Dockerignore{{LocalPath: "b", Patterns: []string{"a"}}}}),
		true,
	},
	{
		"ImageTarget.DockerIgnores equal",
		Manifest{}.WithImageTarget(ImageTarget{dockerignores: []Dockerignore{{LocalPath: "a", Patterns: []string{"b"}}}}),
		Manifest{}.WithImageTarget(ImageTarget{dockerignores: []Dockerignore{{LocalPath: "a", Patterns: []string{"b"}}}}),
		false,
	},
	{
		"DockerCompose.ConfigPaths equal",
		Manifest{}.WithDeployTarget(DockerComposeTarget{ConfigPaths: []string{"/src/docker-compose.yml"}}),
		Manifest{}.WithDeployTarget(DockerComposeTarget{ConfigPaths: []string{"/src/docker-compose.yml"}}),
		false,
	},
	{
		"DockerCompose.ConfigPaths unequal",
		Manifest{}.WithDeployTarget(DockerComposeTarget{ConfigPaths: []string{"/src/docker-compose1.yml"}}),
		Manifest{}.WithDeployTarget(DockerComposeTarget{ConfigPaths: []string{"/src/docker-compose2.yml"}}),
		true,
	},
	{
		"DockerCompose.YAMLRaw equal",
		Manifest{}.WithDeployTarget(DockerComposeTarget{YAMLRaw: []byte("hello world")}),
		Manifest{}.WithDeployTarget(DockerComposeTarget{YAMLRaw: []byte("hello world")}),
		false,
	},
	{
		"DockerCompose.YAMLRaw unequal",
		Manifest{}.WithDeployTarget(DockerComposeTarget{YAMLRaw: []byte("hello world")}),
		Manifest{}.WithDeployTarget(DockerComposeTarget{YAMLRaw: []byte("goodbye world")}),
		true,
	},
	{
		"DockerCompose.DfRaw equal",
		Manifest{}.WithDeployTarget(DockerComposeTarget{DfRaw: []byte("hello world")}),
		Manifest{}.WithDeployTarget(DockerComposeTarget{DfRaw: []byte("hello world")}),
		false,
	},
	{
		"DockerCompose.DfRaw unequal",
		Manifest{}.WithDeployTarget(DockerComposeTarget{DfRaw: []byte("hello world")}),
		Manifest{}.WithDeployTarget(DockerComposeTarget{DfRaw: []byte("goodbye world")}),
		true,
	},
	{
		"k8s.YAML equal",
		Manifest{}.WithDeployTarget(K8sTarget{YAML: "hello world"}),
		Manifest{}.WithDeployTarget(K8sTarget{YAML: "hello world"}),
		false,
	},
	{
		"k8s.YAML unequal",
		Manifest{}.WithDeployTarget(K8sTarget{YAML: "hello world"}),
		Manifest{}.WithDeployTarget(K8sTarget{YAML: "goodbye world"}),
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
		true,
	},
	{
		"TriggerMode equal",
		Manifest{TriggerMode: TriggerModeManual_AutoInit},
		Manifest{TriggerMode: TriggerModeManual_AutoInit},
		false,
	},
	{
		"TriggerMode unequal",
		Manifest{TriggerMode: TriggerModeAuto_AutoInit},
		Manifest{TriggerMode: TriggerModeManual_AutoInit},
		false,
	},
	{
		"Name equal",
		Manifest{Name: "foo"},
		Manifest{Name: "bar"},
		false,
	},
	{
		"Name & k8s YAML unequal",
		Manifest{Name: "foo"}.WithDeployTarget(K8sTarget{YAML: "hello world"}),
		Manifest{Name: "bar"}.WithDeployTarget(K8sTarget{YAML: "goodbye world"}),
		true,
	},
	{
		"LocalTarget equal",
		Manifest{}.WithDeployTarget(NewLocalTarget("foo", ToHostCmdInDir("beep boop", "path/to/tiltfile"), Cmd{}, []string{"bar", "baz"})),
		Manifest{}.WithDeployTarget(NewLocalTarget("foo", ToHostCmdInDir("beep boop", "path/to/tiltfile"), Cmd{}, []string{"bar", "baz"})),
		false,
	},
	{
		"LocalTarget.Name unequal",
		Manifest{}.WithDeployTarget(NewLocalTarget("foo", ToHostCmdInDir("beep boop", "path/to/tiltfile"), Cmd{}, []string{"bar", "baz"})),
		Manifest{}.WithDeployTarget(NewLocalTarget("foooooo", ToHostCmdInDir("beep boop", "path/to/tiltfile"), Cmd{}, []string{"bar", "baz"})),
		true,
	},
	{
		"LocalTarget.UpdateCmd unequal",
		Manifest{}.WithDeployTarget(NewLocalTarget("foo", ToHostCmdInDir("beep boop", "path/to/tiltfile"), Cmd{}, []string{"bar", "baz"})),
		Manifest{}.WithDeployTarget(NewLocalTarget("foo", ToHostCmdInDir("bippity boppity", "path/to/tiltfile"), Cmd{}, []string{"bar", "baz"})),
		true,
	},
	{
		"LocalTarget.workdir unequal",
		Manifest{}.WithDeployTarget(NewLocalTarget("foo", ToHostCmdInDir("beep boop", "path/to/tiltfile"), Cmd{}, []string{"bar", "baz"})),
		Manifest{}.WithDeployTarget(NewLocalTarget("foo", ToHostCmdInDir("beep boop", "some/other/path"), Cmd{}, []string{"bar", "baz"})),
		true,
	},
	{
		"LocalTarget.Deps unequal and doesn't invalidate",
		Manifest{}.WithDeployTarget(NewLocalTarget("foo", ToHostCmd("beep boop"), Cmd{}, []string{"bar", "baz"})),
		Manifest{}.WithDeployTarget(NewLocalTarget("foo", ToHostCmd("beep boop"), Cmd{}, []string{"quux", "baz"})),
		false,
	},
	{
		"CustomBuild.Deps unequal and doesn't invalidate",
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(CustomBuild{Deps: []string{"foo", "bar"}})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(CustomBuild{Deps: []string{"bar", "quux"}})),
		false,
	},
	{
		"DockerBuild.CacheFrom unequal and doesn't invalidate",
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{CacheFrom: []string{"foo", "bar"}})),
		Manifest{}.WithImageTarget(ImageTarget{}.WithBuildDetails(DockerBuild{CacheFrom: []string{"bar", "quux"}})),
		false,
	},
}

func TestManifestEquality(t *testing.T) {
	for _, c := range equalitytests {
		t.Run(c.name, func(t *testing.T) {
			actualInvalidates := ChangesInvalidateBuild(c.m1, c.m2)

			if actualInvalidates != c.expectedInvalidates {
				t.Errorf("Expected m1 -> m2 invalidates build to be %t, but got %t\n\tm1: %+v\n\tm2: %+v", c.expectedInvalidates, actualInvalidates, c.m1, c.m2)
			}
		})
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

func TestHostCmdToString(t *testing.T) {
	cmd := ToHostCmd("echo hi")
	assert.Equal(t, "echo hi", cmd.String())
}

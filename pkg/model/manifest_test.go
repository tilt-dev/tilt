package model

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

var portFwd8000 = []v1alpha1.Forward{{LocalPort: 8080}}
var portFwd8001 = []v1alpha1.Forward{{LocalPort: 8081}}

var img1 = container.MustParseSelector("blorg.io/blorgdev/blorg-frontend:tilt-361d98a2d335373f")
var img2 = container.MustParseSelector("blorg.io/blorgdev/blorg-backend:tilt-361d98a2d335373f")

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
		Manifest{}.WithDeployTarget(K8sTarget{
			KubernetesApplySpec: v1alpha1.KubernetesApplySpec{
				PortForwardTemplateSpec: &v1alpha1.PortForwardTemplateSpec{Forwards: portFwd8000},
			},
		}),
		Manifest{}.WithDeployTarget(K8sTarget{
			KubernetesApplySpec: v1alpha1.KubernetesApplySpec{
				PortForwardTemplateSpec: &v1alpha1.PortForwardTemplateSpec{Forwards: portFwd8001},
			},
		}),
		true,
	},
	{
		"PortForwards equal",
		Manifest{}.WithDeployTarget(K8sTarget{
			KubernetesApplySpec: v1alpha1.KubernetesApplySpec{
				PortForwardTemplateSpec: &v1alpha1.PortForwardTemplateSpec{Forwards: portFwd8000},
			},
		}),
		Manifest{}.WithDeployTarget(K8sTarget{
			KubernetesApplySpec: v1alpha1.KubernetesApplySpec{
				PortForwardTemplateSpec: &v1alpha1.PortForwardTemplateSpec{Forwards: portFwd8000},
			},
		}),
		false,
	},
	{
		"ImageTarget.ImageMapSpec.Selector unequal",
		Manifest{}.WithImageTarget(ImageTarget{ImageMapSpec: v1alpha1.ImageMapSpec{Selector: img1.RefFamiliarString()}}),
		Manifest{}.WithImageTarget(ImageTarget{ImageMapSpec: v1alpha1.ImageMapSpec{Selector: img2.RefFamiliarString()}}),
		true,
	},
	{
		"ImageTarget.ConfigurationRef equal",
		Manifest{}.WithImageTarget(ImageTarget{ImageMapSpec: v1alpha1.ImageMapSpec{Selector: img1.RefFamiliarString()}}),
		Manifest{}.WithImageTarget(ImageTarget{ImageMapSpec: v1alpha1.ImageMapSpec{Selector: img1.RefFamiliarString()}}),
		false,
	},
	{
		"k8s.YAML equal",
		Manifest{}.WithDeployTarget(NewK8sTargetForTesting("hello world")),
		Manifest{}.WithDeployTarget(NewK8sTargetForTesting("hello world")),
		false,
	},
	{
		"k8s.YAML unequal",
		Manifest{}.WithDeployTarget(NewK8sTargetForTesting("hello world")),
		Manifest{}.WithDeployTarget(NewK8sTargetForTesting("goodbye world")),
		true,
	},
	{
		"TriggerMode equal",
		Manifest{TriggerMode: TriggerModeManualWithAutoInit},
		Manifest{TriggerMode: TriggerModeManualWithAutoInit},
		false,
	},
	{
		"TriggerMode unequal",
		Manifest{TriggerMode: TriggerModeAuto},
		Manifest{TriggerMode: TriggerModeManualWithAutoInit},
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
		Manifest{Name: "foo"}.WithDeployTarget(NewK8sTargetForTesting("hello world")),
		Manifest{Name: "bar"}.WithDeployTarget(NewK8sTargetForTesting("goodbye world")),
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
		"labels unequal and doesn't invalidate",
		Manifest{}.WithLabels(map[string]string{"foo": "bar"}),
		Manifest{}.WithLabels(map[string]string{"foo": "baz"}),
		false,
	},
	{
		"Links unequal and doesn't invalidate",
		Manifest{}.WithDeployTarget(NewLocalTarget("foo", Cmd{}, Cmd{}, nil).WithLinks([]Link{
			{Name: "bar", URL: mustURL("mysql://root:password@localhost:3306/mydbname")},
		})),
		Manifest{}.WithDeployTarget(NewLocalTarget("foo", Cmd{}, Cmd{}, nil).WithLinks([]Link{
			// the username is changed because if not properly ignored, that will cause a panic due to url.Userinfo
			// having unexported fields [everything else remains the same to avoid go-cmp short-circuiting elsewhere]
			{Name: "bar", URL: mustURL("mysql://r00t:password@localhost:3306/mydbname")},
		})),
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
		Name: "blah",
		Spec: v1alpha1.DockerComposeServiceSpec{
			Service: "blah",
			Project: v1alpha1.DockerComposeProject{
				ConfigPaths: []string{"docker-compose.yml"},
			},
		},
	}
	err := targ.Validate()
	assert.NoError(t, err)

	noConfPath := DockerComposeTarget{Name: "blah"}
	err = noConfPath.Validate()
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "missing config path")
	}

	noName := DockerComposeTarget{
		Spec: v1alpha1.DockerComposeServiceSpec{
			Service: "blah",
			Project: v1alpha1.DockerComposeProject{
				ConfigPaths: []string{"docker-compose.yml"},
			},
		},
	}
	err = noName.Validate()
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "missing name")
	}
}

func TestHostCmdToString(t *testing.T) {
	cmd := ToHostCmd("echo hi")
	assert.Equal(t, "echo hi", cmd.String())
}

func mustURL(v string) *url.URL {
	u, err := url.Parse(v)
	if err != nil {
		panic(fmt.Errorf("failed to parse URL[%s]: %v", v, err))
	}
	return u
}

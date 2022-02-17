package v1alpha1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestExtension(t *testing.T) {
	f := newFixture(t)

	f.File("Tiltfile", `
v1alpha1.extension_repo(name='default', url='https://github.com/tilt-dev/tilt-extensions', ref='HEAD')
v1alpha1.extension(name='cancel', repo_name='default', repo_path='cancel')
`)
	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	set := MustState(result)

	repo := set.GetSetForType(&v1alpha1.ExtensionRepo{})["default"].(*v1alpha1.ExtensionRepo)
	require.NotNil(t, repo)
	require.Equal(t, "https://github.com/tilt-dev/tilt-extensions", repo.Spec.URL)
	require.Equal(t, "HEAD", repo.Spec.Ref)

	ext := set.GetSetForType(&v1alpha1.Extension{})["cancel"].(*v1alpha1.Extension)
	require.NotNil(t, ext)
	require.Equal(t, "default", ext.Spec.RepoName)
}

func TestExtensionArgs(t *testing.T) {
	f := newFixture(t)

	f.File("Tiltfile", `
v1alpha1.extension_repo(name='default', url='https://github.com/tilt-dev/tilt-extensions', ref='HEAD')
v1alpha1.extension(name='cancel', repo_name='default', repo_path='cancel', args=['--namespace=foo'])
`)
	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	set := MustState(result)

	ext := set.GetSetForType(&v1alpha1.Extension{})["cancel"].(*v1alpha1.Extension)
	require.NotNil(t, ext)
	require.Equal(t, []string{"--namespace=foo"}, ext.Spec.Args)
}

func TestExtensionValidation(t *testing.T) {
	f := newFixture(t)

	f.File("Tiltfile", `
v1alpha1.extension_repo(name='default', url='ftp://github.com/tilt-dev/tilt-extensions')
`)
	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), "URLs must start with http(s):// or file://")
}

func TestFileWatchAsDict(t *testing.T) {
	f := newFixture(t)

	f.File("Tiltfile", `
v1alpha1.file_watch(name='my-fw', watched_paths=['./dir'], ignores=[{'base_path': './dir/ignore', 'patterns': ['**']}])
`)
	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	set := MustState(result)

	fw := set.GetSetForType(&v1alpha1.FileWatch{})["my-fw"].(*v1alpha1.FileWatch)
	require.NotNil(t, fw)
	require.Equal(t, fw.Spec, v1alpha1.FileWatchSpec{
		WatchedPaths: []string{f.JoinPath("dir")},
		Ignores: []v1alpha1.IgnoreDef{
			v1alpha1.IgnoreDef{
				BasePath: f.JoinPath("dir", "ignore"),
				Patterns: []string{"**"},
			},
		},
	})
}

func TestFileWatchDisableOn(t *testing.T) {
	f := newFixture(t)

	f.File("Tiltfile", `
v1alpha1.file_watch(name='my-fw',
                    watched_paths=['./dir'],
                    disable_source={'config_map':{'name':'my-fw','key':'disabled'}})
`)
	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	set := MustState(result)

	fw := set.GetSetForType(&v1alpha1.FileWatch{})["my-fw"].(*v1alpha1.FileWatch)
	require.NotNil(t, fw)
	require.Equal(t, fw.Spec, v1alpha1.FileWatchSpec{
		WatchedPaths: []string{f.JoinPath("dir")},
		DisableSource: &v1alpha1.DisableSource{
			ConfigMap: &v1alpha1.ConfigMapDisableSource{
				Name: "my-fw",
				Key:  "disabled",
			},
		},
	})
}

func TestFileWatchWithIgnoreBuiltin(t *testing.T) {
	f := newFixture(t)

	f.File("Tiltfile", `
v1alpha1.file_watch(
  name='my-fw',
  watched_paths=['./dir'],
  ignores=[v1alpha1.ignore_def(base_path='./dir/ignore', patterns=['**'])])
`)
	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	set := MustState(result)

	fw := set.GetSetForType(&v1alpha1.FileWatch{})["my-fw"].(*v1alpha1.FileWatch)
	require.NotNil(t, fw)
	require.Equal(t, fw.Spec, v1alpha1.FileWatchSpec{
		WatchedPaths: []string{f.JoinPath("dir")},
		Ignores: []v1alpha1.IgnoreDef{
			v1alpha1.IgnoreDef{
				BasePath: f.JoinPath("dir", "ignore"),
				Patterns: []string{"**"},
			},
		},
	})
}

func TestCmdDefaultDir(t *testing.T) {
	f := newFixture(t)

	f.File("Tiltfile", `
v1alpha1.cmd(
  name='my-cmd',
  args=['echo', 'hello'])
`)
	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	set := MustState(result)

	cmd := set.GetSetForType(&v1alpha1.Cmd{})["my-cmd"].(*v1alpha1.Cmd)
	require.NotNil(t, cmd)
	require.Equal(t, cmd.Spec, v1alpha1.CmdSpec{
		Args: []string{"echo", "hello"},
		Dir:  f.Path(),
	})
}

func TestUIButton(t *testing.T) {
	f := newFixture(t)

	f.File("Tiltfile", `
v1alpha1.ui_button(
  name='my-button',
  annotations={'tilt.dev/resource': 'fe', 'tilt.dev/log-span-id': 'fe'},
  text='hello world',
  icon_name='circle',
  location={'component_type': 'resource', 'component_id': 'fe'})
`)
	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	set := MustState(result)

	obj := set.GetSetForType(&v1alpha1.UIButton{})["my-button"].(*v1alpha1.UIButton)
	require.NotNil(t, obj)
	require.Equal(t, obj, &v1alpha1.UIButton{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-button",
			Annotations: map[string]string{"tilt.dev/resource": "fe", "tilt.dev/log-span-id": "fe"},
		},
		Spec: v1alpha1.UIButtonSpec{
			Text:     "hello world",
			IconName: "circle",
			Location: v1alpha1.UIComponentLocation{ComponentType: "resource", ComponentID: "fe"},
		},
	})
}

func TestKubernetesDiscoveryu(t *testing.T) {
	f := newFixture(t)

	f.File("Tiltfile", `
v1alpha1.kubernetes_discovery(
  name='my-discovery',
  annotations={'tilt.dev/resource': 'fe'},
  extra_selectors=[{'match_labels': {'app': 'fe'}}])
`)
	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	set := MustState(result)

	obj := set.GetSetForType(&v1alpha1.KubernetesDiscovery{})["my-discovery"].(*v1alpha1.KubernetesDiscovery)
	require.NotNil(t, obj)
	require.Equal(t, obj, &v1alpha1.KubernetesDiscovery{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-discovery",
			Annotations: map[string]string{"tilt.dev/resource": "fe"},
		},
		Spec: v1alpha1.KubernetesDiscoverySpec{
			ExtraSelectors: k8s.SetsAsLabelSelectors([]labels.Set{{"app": "fe"}}),
		},
	})
}

func TestConfigMap(t *testing.T) {
	f := newFixture(t)

	f.File("Tiltfile", `
v1alpha1.config_map(
  name='my-config',
  labels={'bar': 'baz'},
  data={'foo': 'bar'})
`)
	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	set := MustState(result)

	obj := set.GetSetForType(&v1alpha1.ConfigMap{})["my-config"].(*v1alpha1.ConfigMap)
	require.NotNil(t, obj)
	require.Equal(t, obj, &v1alpha1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "my-config",
			Labels: map[string]string{"bar": "baz"},
		},
		Data: map[string]string{"foo": "bar"},
	})
}

func TestKubernetesApply(t *testing.T) {
	f := newFixture(t)

	f.File("Tiltfile", `

config="""
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config-map
data:
  foo: bar
"""

v1alpha1.kubernetes_apply(
  name='my-apply',
  discovery_strategy='selectors-only',
  timeout='2s',
  yaml=config)
`)
	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	set := MustState(result)

	obj := set.GetSetForType(&v1alpha1.KubernetesApply{})["my-apply"].(*v1alpha1.KubernetesApply)
	require.NotNil(t, obj)
	require.Equal(t, obj, &v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-apply",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			DiscoveryStrategy: "selectors-only",
			Timeout:           metav1.Duration{Duration: 2 * time.Second},
			YAML: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config-map
data:
  foo: bar
`,
		},
	})
}

func newFixture(tb testing.TB) *starkit.Fixture {
	return starkit.NewFixture(tb, NewPlugin())
}

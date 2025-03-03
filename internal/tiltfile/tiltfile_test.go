package tiltfile

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/tilt-dev/clusterid"
	tiltanalytics "github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/apis/liveupdate"
	ctrltiltfile "github.com/tilt-dev/tilt/internal/controllers/apis/tiltfile"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/feature"
	"github.com/tilt-dev/tilt/internal/ignore"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/localexec"
	"github.com/tilt-dev/tilt/internal/ospath"
	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/internal/tiltfile/cisettings"
	"github.com/tilt-dev/tilt/internal/tiltfile/config"
	"github.com/tilt-dev/tilt/internal/tiltfile/hasher"
	tiltfile_k8s "github.com/tilt-dev/tilt/internal/tiltfile/k8s"
	"github.com/tilt-dev/tilt/internal/tiltfile/k8scontext"
	"github.com/tilt-dev/tilt/internal/tiltfile/testdata"
	"github.com/tilt-dev/tilt/internal/tiltfile/tiltextension"
	"github.com/tilt-dev/tilt/internal/tiltfile/version"
	"github.com/tilt-dev/tilt/internal/yaml"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/wmclient/pkg/analytics"
)

type localResourceLinks []model.Link
type k8sResourceLinks []model.Link
type dcResourceLinks []model.Link

const simpleDockerfile = "FROM golang:1.10"

const simpleDockerignore = "build/"

func TestNoTiltfile(t *testing.T) {
	f := newFixture(t)

	f.loadErrString("No Tiltfile found at")
	f.assertConfigFiles("Tiltfile")
}

func TestEmpty(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", "")
	f.load()
}

func TestMissingDockerfile(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_resource('foo', 'foo.yaml')
`)

	f.loadErrString(filepath.Join("foo", "Dockerfile"), testutils.IsNotExistMessage(), "error reading dockerfile")
}

func TestCustomBuildBadMethodCall(t *testing.T) {
	f := newFixture(t)
	f.setupFoo()
	f.file("Tiltfile", `
hfb = custom_build(
  'gcr.io/foo',
  'docker build -t $TAG foo',
  ['foo']
).asdf()
`)

	f.loadErrString("Error: custom_build has no .asdf field or method")
}

func TestSimple(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
`)

	f.load("foo")

	m := f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "foo.yaml")

	iTarget := m.ImageTargetAt(0)

	// Make sure there's no live update in the default case.
	assert.True(t, iTarget.IsDockerBuild())
	assert.True(t, liveupdate.IsEmptySpec(iTarget.LiveUpdateSpec))
}

// I.e. make sure that we handle de/normalization between `fooimage` <--> `docker.io/library/fooimage`
func TestLocalImageRef(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("foo/Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("fooimage")))

	f.file("Tiltfile", `

docker_build('fooimage', 'foo')
k8s_yaml('foo.yaml')
`)

	f.load()

	f.assertNextManifest("foo",
		db(image("fooimage")),
		deployment("foo"))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "foo.yaml")
}

func TestExplicitDockerfileIsConfigFile(t *testing.T) {
	f := newFixture(t)
	f.setupFoo()
	f.dockerfile("other/Dockerfile")
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo', dockerfile='other/Dockerfile')
k8s_yaml('foo.yaml')
`)
	f.load()
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo.yaml", "other/Dockerfile", "foo/.dockerignore")
}

func TestDockerfileNone(t *testing.T) {
	f := newFixture(t)
	f.setupFoo()
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo', dockerfile=None)
k8s_yaml('foo.yaml')
`)
	f.load()
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo.yaml", "foo/Dockerfile", "foo/.dockerignore")
}

func TestExplicitDockerfileAsLocalPath(t *testing.T) {
	f := newFixture(t)
	f.setupFoo()
	f.dockerfile("other/Dockerfile")
	f.file("Tiltfile", `
r = local_git_repo('.')
docker_build('gcr.io/foo', 'foo', dockerfile=r.paths('other/Dockerfile'))
k8s_yaml('foo.yaml')
`)
	f.load()
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo.yaml", "other/Dockerfile", "foo/.dockerignore")
}

func TestExplicitDockerfileContents(t *testing.T) {
	f := newFixture(t)
	f.setupFoo()
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo', dockerfile_contents='FROM alpine')
k8s_yaml('foo.yaml')
`)
	f.load()
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo.yaml", "foo/.dockerignore")
	f.assertNextManifest("foo", db(image("gcr.io/foo")))
}

func TestExplicitDockerfileContentsAsBlob(t *testing.T) {
	f := newFixture(t)
	f.setupFoo()
	f.dockerfile("other/Dockerfile")
	f.file("Tiltfile", `
df = read_file('other/Dockerfile')
docker_build('gcr.io/foo', 'foo', dockerfile_contents=df)
k8s_yaml('foo.yaml')
`)
	f.load()
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo.yaml", "other/Dockerfile", "foo/.dockerignore")
	f.assertNextManifest("foo", db(image("gcr.io/foo")))
}

func TestCantSpecifyDFPathAndContents(t *testing.T) {
	f := newFixture(t)
	f.setupFoo()
	f.dockerfile("other/Dockerfile")
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo', dockerfile_contents='FROM alpine', dockerfile='foo/Dockerfile')
k8s_yaml('foo.yaml')
`)

	f.loadErrString("Cannot specify both dockerfile and dockerfile_contents")
}

func TestVerifiesGitRepo(t *testing.T) {
	f := newFixture(t)
	f.file("Tiltfile", "local_git_repo('.')")
	f.loadErrString("isn't a valid git repo")
}

func TestLocal(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
cmd = 'cat foo.yaml'
if os.name == 'nt':
  cmd = 'type foo.yaml'
yaml = local(cmd)
k8s_yaml(yaml)
`)

	f.load()

	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))

	cmdStr := "cat foo.yaml"
	if runtime.GOOS == "windows" {
		cmdStr = "type foo.yaml"
	}
	assert.Contains(t, f.out.String(), "local: "+cmdStr)
	assert.Contains(t, f.out.String(), " → kind: Deployment")
}

func TestLocalBat(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
yaml = local(command='cat foo.yaml', command_bat='type foo.yaml')
k8s_yaml(yaml)
`)

	f.load()

	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))

	cmdStr := "cat foo.yaml"
	if runtime.GOOS == "windows" {
		cmdStr = "type foo.yaml"
	}
	assert.Contains(t, f.out.String(), "local: "+cmdStr)
	assert.Contains(t, f.out.String(), " → kind: Deployment")
}

func TestLocalEnv(t *testing.T) {
	f := newFixture(t)

	// contrived example to ensure that the environment is correctly passed to local -- an env var is echoed back out
	// which then gets passed as an ignore so that it's visible in the load result for assertion
	f.file("Tiltfile", `
ignore = str(local('echo $FOO', command_bat='echo %FOO%', env={'FOO': 'bar'})).rstrip('\r\n')
watch_settings(ignore=ignore)
`)

	f.load()

	assert.Equal(t, []string{"bar"}, f.loadResult.WatchSettings.Ignores[0].Patterns)
}

func TestLocalEmptyArray(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local([])
`)

	f.loadErrString("empty cmd")
}

func TestLocalEmptyString(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local('')
`)

	f.loadErrString("empty cmd")
}

func TestLocalStdin(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local('head -4 | tail -2', stdin='''foo
bar
baz
quu
qux
''')
`)

	f.load()
	require.Contains(t, f.out.String(), `head -4 | tail -2
 → baz
 → quu`)
}

func TestLocalStdinChain(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}

	f := newFixture(t)

	f.file("Tiltfile", `
local('cat', stdin=local('echo hi'))
`)

	f.load()
	require.Contains(t, f.out.String(), "local: echo hi\n → hi\nlocal: cat\n → hi")
}

func TestCustomBuildBat(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
custom_build('gcr.io/foo', command='unix build', command_bat='windows build', deps=[])
k8s_yaml('foo.yaml')
`)

	f.load()

	args := "unix build"
	if runtime.GOOS == "windows" {
		args = "windows build"
	}
	f.assertNextManifest("foo",
		cb(
			image("gcr.io/foo"),
			cmd(args, f.Path()),
		),
		deployment("foo"))
}

func TestLocalQuiet(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
local('echo foobar', quiet=True)
`)

	f.load()

	assert.Contains(t, f.out.String(), "local: echo foobar")
	assert.NotContains(t, f.out.String(), " → foobar")
}

func TestLocalEchoOff(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
local('echo foobar', echo_off=True)
`)

	f.load()

	assert.NotContains(t, f.out.String(), "local: echo foobar")
}

func TestLocalNoOutput(t *testing.T) {
	type tc struct {
		echoOff               bool
		quiet                 bool
		shouldDisplayNoOutput bool
	}

	// only if BOTH quiet=False + echo_off=False should the [no output] show up
	// 	* if quiet=True, we don't care about output, so doesn't make sense to log that there was NO output
	// 	* if echo_off=True, we don't know what command it's coming from, so it's more confusing than helpful
	tcs := []tc{
		{echoOff: true, quiet: true, shouldDisplayNoOutput: false},
		{echoOff: false, quiet: true, shouldDisplayNoOutput: false},
		{echoOff: true, quiet: true, shouldDisplayNoOutput: false},
		{echoOff: false, quiet: false, shouldDisplayNoOutput: true},
	}

	goBoolToStarlark := func(v bool) string {
		if v {
			return "True"
		}
		return "False"
	}

	for _, tc := range tcs {
		name := fmt.Sprintf("EchoOff%s_Quiet%s", goBoolToStarlark(tc.echoOff), goBoolToStarlark(tc.quiet))
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)

			f.setupFoo()

			f.file(
				"Tiltfile", fmt.Sprintf(`
local('exit 0', echo_off=%s, quiet=%s)
`, goBoolToStarlark(tc.echoOff), goBoolToStarlark(tc.quiet)))

			f.load()

			out := f.out.String()
			if !tc.echoOff {
				assert.Contains(t, out, "local: exit 0")
			} else {
				assert.NotContains(t, out, "exit")
			}

			if tc.shouldDisplayNoOutput {
				assert.Contains(t, out, "[no output]")
			} else {
				assert.NotContains(t, out, "no output")
			}
		})
	}
}

func TestLocalArgvCmd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows doesn't support argv commands. Go converts it to a single string")
	}
	f := newFixture(t)

	// this would generate a syntax error if evaluated by a shell
	f.file("Tiltfile", `local(['echo', 'a"b'])`)
	f.load()

	assert.Contains(t, f.out.String(), `a"b`)
}

func TestLocalTiltEnvPropagation(t *testing.T) {
	f := newFixture(t)

	doTest := func(t testing.TB, expectedHost string, expectedPort int) {
		t.Helper()

		f.file("Tiltfile", `
local(command='echo Tilt host is $TILT_HOST', command_bat='echo Tilt host is %TILT_HOST%', echo_off=True)
local(command='echo Tilt port is $TILT_PORT', command_bat='echo Tilt port is %TILT_PORT%', echo_off=True)
`)
		f.load()

		assert.Contains(t, f.out.String(), fmt.Sprintf(`Tilt host is %s`, expectedHost))
		assert.Contains(t, f.out.String(), fmt.Sprintf(`Tilt port is %d`, expectedPort))
	}

	t.Run("Implicit", func(t *testing.T) {
		os.Unsetenv("TILT_HOST")
		os.Unsetenv("TILT_PORT")
		// $TILT_HOST + $TILT_PORT are not explicitly defined anywhere in the test fixture but should be
		// auto-populated (hardcoded to 1.2.3.4/12345 for tests - no real apiserver is actually loaded)
		f.webHost = "1.2.3.4"
		doTest(t, "1.2.3.4", 12345)
	})

	t.Run("Explicit", func(t *testing.T) {
		t.Setenv("TILT_HOST", "7.8.9.0")
		t.Setenv("TILT_PORT", "7890")

		// if values were explicitly passed (e.g. `local('...', env={"TILT_PORT": 7890})`, they should be respected
		doTest(t, "7.8.9.0", 7890)
	})
}

func TestReadFile(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
yaml = read_file('foo.yaml')
k8s_yaml(yaml)
`)

	f.load()

	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "foo.yaml")
}

func TestKustomize(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.file("kustomization.yaml", kustomizeFileText)
	f.file("configMap.yaml", kustomizeConfigMapText)
	f.file("deployment.yaml", kustomizeDeploymentText)
	f.file("service.yaml", kustomizeServiceText)
	f.file("Tiltfile", `

docker_build("gcr.io/foo", "foo")
k8s_yaml(kustomize("."))
k8s_resource("the-deployment", "foo")
`)
	f.load()
	f.assertNextManifest("foo", deployment("the-deployment"), numEntities(2))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "configMap.yaml", "deployment.yaml", "kustomization.yaml", "service.yaml")
}

func TestKustomizeFlags(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.file("kustomization.yaml", kustomizeFileText)
	f.file("configMap.yaml", kustomizeConfigMapText)
	f.file("deployment.yaml", kustomizeDeploymentText)
	f.file("service.yaml", kustomizeServiceText)
	f.file("Tiltfile", `

docker_build("gcr.io/foo", "foo")
k8s_yaml(kustomize(".", flags=['--enable-helm']))
k8s_resource("the-deployment", "foo")
`)
	f.load()
	f.assertNextManifest("foo", deployment("the-deployment"), numEntities(2))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "configMap.yaml", "deployment.yaml", "kustomization.yaml", "service.yaml")
	assert.Contains(t, f.out.String(), "kustomize build --enable-helm")
}

func TestKustomizeBin(t *testing.T) {
	f := newFixture(t)
	f.file("kustomization.yaml", kustomizeFileText)
	f.file("configMap.yaml", kustomizeConfigMapText)
	f.file("deployment.yaml", kustomizeDeploymentText)
	f.file("service.yaml", kustomizeServiceText)
	sentinel := f.WriteFile("kustomize.txt", "")
	var wrapper string
	if runtime.GOOS == "windows" {
		wrapper = f.WriteFile("kustomize.bat", fmt.Sprintf(`@echo off
echo %%* > %s
kustomize.exe %%*
`, sentinel))
		// convert backslashes in path
		wrapper = strings.ReplaceAll(wrapper, "\\", "/")
	} else {
		wrapper = f.WriteFile("kustomize", fmt.Sprintf(`#!/bin/sh
echo "$@" > %s
exec kustomize "$@"
`, sentinel))
		_ = os.Chmod(wrapper, 0755)
	}

	f.file("Tiltfile", fmt.Sprintf(`
k8s_yaml(kustomize(".", kustomize_bin="%s"))
k8s_resource("the-deployment", "foo")
`, wrapper))
	f.load()
	sentinelContents, err := os.ReadFile(sentinel)
	assert.Nil(t, err)
	assert.EqualValues(t, "build .", strings.Trim(string(sentinelContents), " \r\n"))
}

func TestKustomizeError(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", "kustomize('.')")
	f.loadErrString("unable to find one of 'kustomization.yaml', 'kustomization.yml' or 'Kustomization'")
}

func TestKustomization(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.file("Kustomization", kustomizeFileText)
	f.file("configMap.yaml", kustomizeConfigMapText)
	f.file("deployment.yaml", kustomizeDeploymentText)
	f.file("service.yaml", kustomizeServiceText)
	f.file("Tiltfile", `

docker_build("gcr.io/foo", "foo")
k8s_yaml(kustomize("."))
k8s_resource("the-deployment", "foo")
`)
	f.load()
	f.assertNextManifest("foo", deployment("the-deployment"), numEntities(2))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "configMap.yaml", "deployment.yaml", "Kustomization", "service.yaml")
}

func TestDockerBuildTarget(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build("gcr.io/foo", "foo", target='stage')
`)
	f.load()
	m := f.assertNextManifest("foo")
	assert.Equal(t, "stage", m.ImageTargets[0].BuildDetails.(model.DockerBuild).Target)
}

func TestDockerBuildSSH(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build("gcr.io/foo", "foo", ssh='default')
`)
	f.load()
	m := f.assertNextManifest("foo")
	assert.Equal(t, []string{"default"}, m.ImageTargets[0].BuildDetails.(model.DockerBuild).SSHAgentConfigs)
}

func TestDockerBuildSecret(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build("gcr.io/foo", "foo", secret='id=shibboleth')
`)
	f.load()
	m := f.assertNextManifest("foo")
	assert.Equal(t, []string{"id=shibboleth"}, m.ImageTargets[0].BuildDetails.(model.DockerBuild).Secrets)
}

func TestDockerBuildNetwork(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build("gcr.io/foo", "foo", network='default')
`)
	f.load()
	m := f.assertNextManifest("foo")
	assert.Equal(t, "default", m.ImageTargets[0].BuildDetails.(model.DockerBuild).Network)
}

func TestDockerBuildPull(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build("gcr.io/foo", "foo", pull=True)
`)
	f.load()
	m := f.assertNextManifest("foo")
	assert.True(t, m.ImageTargets[0].BuildDetails.(model.DockerBuild).Pull)
}

func TestDockerBuildCacheFrom(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build("gcr.io/foo", "foo", cache_from='gcr.io/foo')
`)
	f.load()
	m := f.assertNextManifest("foo")
	assert.Equal(t, []string{"gcr.io/foo"}, m.ImageTargets[0].BuildDetails.(model.DockerBuild).CacheFrom)
}

func TestDockerBuildExtraTagString(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build("gcr.io/foo", "foo", extra_tag='foo:latest')
`)
	f.load()
	m := f.assertNextManifest("foo")
	assert.Equal(t, []string{"foo:latest"},
		m.ImageTargets[0].BuildDetails.(model.DockerBuild).ExtraTags)
}

func TestDockerBuildExtraTagList(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build("gcr.io/foo", "foo", extra_tag=['foo:latest', 'foo:jenkins-1234'])
`)
	f.load()
	m := f.assertNextManifest("foo")
	assert.Equal(t, []string{"foo:latest", "foo:jenkins-1234"},
		m.ImageTargets[0].BuildDetails.(model.DockerBuild).ExtraTags)
}

func TestDockerBuildExtraTagListInvalid(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build("gcr.io/foo", "foo", extra_tag='cherry bomb')
`)
	f.loadErrString("Argument extra_tag=\"cherry bomb\" not a valid image reference: invalid reference format")
}

func TestDockerBuildCache(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build("gcr.io/foo", "foo", cache='/paths/to/cache')
`)
	f.loadAssertWarnings(cacheObsoleteWarning)
}

func TestK8sResourceAdditiveLinks(t *testing.T) {
	f := newFixture(t)

	f.setupExpand()
	f.file("Tiltfile", `

k8s_yaml('all.yaml')
k8s_resource('a', links=['http://demo-a.localhost/'])
k8s_resource('a')
k8s_resource('b', links=['http://demo-b.localhost/'])
k8s_resource('b', links=['http://demo-b.localhost/api'])
`)
	f.load()
	f.assertNextManifest("a",
		k8sResourceLinks{model.MustNewLink("http://demo-a.localhost/", "")})
	f.assertNextManifest("b",
		k8sResourceLinks{
			model.MustNewLink("http://demo-b.localhost/", ""),
			model.MustNewLink("http://demo-b.localhost/api", ""),
		})
}

func TestDuplicateImageNames(t *testing.T) {
	f := newFixture(t)

	f.setupExpand()
	f.file("Tiltfile", `
k8s_yaml('all.yaml')
docker_build('gcr.io/a', 'a')
docker_build('gcr.io/a', 'a')
`)

	f.loadErrString("Image for ref \"gcr.io/a\" has already been defined")
}

func TestInvalidImageNameInDockerBuild(t *testing.T) {
	f := newFixture(t)

	f.setupExpand()
	f.file("Tiltfile", `
k8s_yaml('all.yaml')
docker_build("ceci n'est pas une valid image ref", 'a')
`)

	f.loadErrString("invalid reference format")
}

func TestInvalidImageNameInK8SYAML(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
yaml_str = """
kind: Pod
apiVersion: v1
metadata:
  name: test-pod
spec:
  containers:
  - image: IMAGE_URL
"""

k8s_yaml([blob(yaml_str)])`)

	f.loadErrString("invalid reference format", "test-pod", "IMAGE_URL")
}

type portForwardCase struct {
	name     string
	expr     string
	expected []model.PortForward
	errorMsg string
	webHost  model.WebHost
}

func newPortForwardSuccessCase(name, expr string, expected []model.PortForward) portForwardCase {
	return portForwardCase{name: name, expr: expr, expected: expected}
}

func newPortForwardErrorCase(name, expr, errorMsg string) portForwardCase {
	return portForwardCase{name: name, expr: expr, errorMsg: errorMsg}
}

type resourceLinkCase struct {
	name     string
	expr     string
	expected []model.Link
	errorMsg string
}

func newResourceLinkSuccessCase(name, expr string, expected []model.Link) resourceLinkCase {
	return resourceLinkCase{name: name, expr: expr, expected: expected}
}

func newResourceLinkErrorCase(name, expr, errorMsg string) resourceLinkCase {
	return resourceLinkCase{name: name, expr: expr, errorMsg: errorMsg}
}

func TestPortForward(t *testing.T) {
	portForwardCases := []portForwardCase{
		// int values
		newPortForwardSuccessCase("value_int_local", "8000", []model.PortForward{{LocalPort: 8000}}),
		newPortForwardErrorCase("value_int_local_negative", "-1", "not in the valid range"),
		newPortForwardErrorCase("value_int_local_large", "8000000", "not in the valid range"),

		// string values
		newPortForwardSuccessCase("value_string_local", "'10000'", []model.PortForward{{LocalPort: 10000}}),
		newPortForwardSuccessCase("value_string_both", "'10000:8000'", []model.PortForward{{LocalPort: 10000, ContainerPort: 8000}}),
		newPortForwardErrorCase("value_string_garbage", "'garbage'", "not in the valid range"),
		newPortForwardErrorCase("value_string_empty", "''", "not in the valid range"),

		// PortForward values (via constructor)
		newPortForwardSuccessCase("value_constructor_local", "port_forward(8001)", []model.PortForward{{LocalPort: 8001}}),
		newPortForwardSuccessCase("value_constructor_local_named", "port_forward(8001, name='foo')", []model.PortForward{{LocalPort: 8001, Name: "foo"}}),
		newPortForwardSuccessCase("value_constructor_local_path", "port_forward(8001, link_path='v1/ui')",
			[]model.PortForward{model.MustPortForward(8001, 0, "", "", "v1/ui")}),
		newPortForwardSuccessCase("value_constructor_both", "port_forward(8001, 443)", []model.PortForward{{LocalPort: 8001, ContainerPort: 443}}),
		newPortForwardSuccessCase("value_constructor_both_named", "port_forward(8001, 443, name='foo')", []model.PortForward{{LocalPort: 8001, ContainerPort: 443, Name: "foo"}}),
		newPortForwardSuccessCase("value_constructor_all_positional", "port_forward(8001, 443, 'foo', 'v1/ui', 'elastic.local')",
			[]model.PortForward{model.MustPortForward(8001, 443, "elastic.local", "foo", "v1/ui")}),
		newPortForwardErrorCase("value_constructor_no_local_port", "port_forward(container_port=443)", "missing argument for local_port"),
		newPortForwardErrorCase("value_constructor_local_port_wrong_type", "port_forward('8001')", "for parameter local_port: got string, want int"),
		newPortForwardErrorCase("value_constructor_bad_path", "port_forward(8001, 443, link_path='invalid_escape%')", "invalid URL escape"),
		newPortForwardErrorCase("value_constructor_name_wrong_type", "port_forward(8001, 443, 54321)", "for parameter name: got int, want string"),
		newPortForwardSuccessCase("value_constructor_host", "port_forward(8001, 443, host='elastic.local')",
			[]model.PortForward{{LocalPort: 8001, ContainerPort: 443, Host: "elastic.local"}}),
		newPortForwardErrorCase("value_constructor_host_wrong_type", "port_forward(8001, 443, host=54321)", "for parameter \"host\": got int, want string"),

		// list values
		newPortForwardSuccessCase("list_mixed", "[8000, port_forward(8001, 443), '8002', '8003:444'],", []model.PortForward{{LocalPort: 8000}, {LocalPort: 8001, ContainerPort: 443}, {LocalPort: 8002}, {LocalPort: 8003, ContainerPort: 444}}),

		// parsing host
		newPortForwardErrorCase("value_host_bad", "'bad+host:10000:8000'", "not a valid hostname or IP address"),
		newPortForwardSuccessCase("value_host_good_ip", "'0.0.0.0:10000:8000'", []model.PortForward{{LocalPort: 10000, ContainerPort: 8000, Host: "0.0.0.0"}}),
		newPortForwardSuccessCase("value_host_good_domain", "'tilt.dev:10000:8000'", []model.PortForward{{LocalPort: 10000, ContainerPort: 8000, Host: "tilt.dev"}}),
		portForwardCase{name: "default_web_host", expr: "8000", webHost: "0.0.0.0",
			expected: []model.PortForward{{LocalPort: 8000, Host: "0.0.0.0"}}},
		portForwardCase{name: "override_web_host", expr: "'tilt.dev:10000:8000'", webHost: "0.0.0.0",
			expected: []model.PortForward{{LocalPort: 10000, ContainerPort: 8000, Host: "tilt.dev"}}},

		// None
		newPortForwardSuccessCase("none", "None", []model.PortForward{}),
		newPortForwardSuccessCase("empty_array", "[]", []model.PortForward{}),
	}

	for _, c := range portForwardCases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)

			f.webHost = c.webHost
			f.setupFoo()
			s := `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_resource('foo', port_forwards=EXPR)
`
			s = strings.ReplaceAll(s, "EXPR", c.expr)
			f.file("Tiltfile", s)

			if c.errorMsg != "" {
				f.loadErrString(c.errorMsg)
				return
			}

			f.load()
			f.assertNextManifest("foo",
				c.expected,
				db(image("gcr.io/foo")),
				deployment("foo"))
		})
	}
}

func TestResourceLinks(t *testing.T) {
	cases := []resourceLinkCase{
		newResourceLinkErrorCase("invalid_type", "123",
			"Want a string, a link, or a sequence of these; found 123"),

		newResourceLinkSuccessCase("value_string", "'http://www.zombo.com'",
			[]model.Link{model.MustNewLink("http://www.zombo.com", "")}),
		newResourceLinkSuccessCase("value_string_adds_scheme", "'www.zombo.com'",
			[]model.Link{model.MustNewLink("http://www.zombo.com", "")}),
		newResourceLinkSuccessCase("value_string_preserves_nonhttp_scheme", "'ws://www.zombo.com'",
			[]model.Link{model.MustNewLink("ws://www.zombo.com", "")}),
		newResourceLinkErrorCase("value_string_empty_url", "''", "url empty"),

		newResourceLinkSuccessCase("value_link_named", "link('https://www.zombo.com', name='zombo')",
			[]model.Link{model.MustNewLink("https://www.zombo.com", "zombo")}),
		newResourceLinkSuccessCase("value_link_unnamed", "link('https://www.zombo.com')",
			[]model.Link{model.MustNewLink("https://www.zombo.com", "")}),
		newResourceLinkSuccessCase("value_link_positional_args", "link('https://www.zombo.com', 'zombo')",
			[]model.Link{model.MustNewLink("https://www.zombo.com", "zombo")}),
		newResourceLinkSuccessCase("link_constructor_adds_scheme", "link('www.zombo.com', 'zombo')",
			[]model.Link{model.MustNewLink("http://www.zombo.com", "zombo")}),
		newResourceLinkErrorCase("link_constructor_requires_URL", "link(name='zombo')",
			"link: missing argument for url"),
		newResourceLinkErrorCase("link_constructor_empty_URL", "link('')",
			"url empty"),

		newResourceLinkSuccessCase("value_list_strings", "['https://www.apple.edu', 'https://www.zombo.com']",
			[]model.Link{model.MustNewLink("https://www.apple.edu", ""), model.MustNewLink("https://www.zombo.com", "")}),
		newResourceLinkSuccessCase("list_strings_add_scheme", "['www.apple.edu', 'www.zombo.com']",
			[]model.Link{model.MustNewLink("http://www.apple.edu", ""), model.MustNewLink("http://www.zombo.com", "")}),
		newResourceLinkSuccessCase("value_list_links",
			"[link('www.apple.edu'), link('www.zombo.com', 'zombo')]",
			[]model.Link{model.MustNewLink("http://www.apple.edu", ""), model.MustNewLink("http://www.zombo.com", "zombo")}),
		newResourceLinkSuccessCase("value_list_,mixed",
			"['www.apple.edu', link('www.zombo.com', 'zombo')]",
			[]model.Link{model.MustNewLink("http://www.apple.edu", ""), model.MustNewLink("http://www.zombo.com", "zombo")}),
		newResourceLinkErrorCase("link_bad_type", "['www.apple.edu', 123]",
			"Want a string, a link, or a sequence of these; found 123"),
	}

	for _, c := range cases {
		t.Run("LocalResource-"+c.name, func(t *testing.T) {
			f := newFixture(t)

			tiltfile := fmt.Sprintf(`
local_resource('foo', 'echo hi', links=%s)
`, c.expr)
			f.file("Tiltfile", tiltfile)

			if c.errorMsg != "" {
				f.loadErrString(c.errorMsg)
				return
			}

			f.load()
			f.assertNextManifest("foo",
				localResourceLinks(c.expected),
				localTarget(updateCmd(f.Path(), "echo hi", nil)),
			)
		})

		t.Run("K8s-"+c.name, func(t *testing.T) {
			f := newFixture(t)

			f.setupFoo()
			s := `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_resource('foo', links=EXPR)
k8s_resource('foo') # test that subsequent calls don't clear the links
`

			s = strings.ReplaceAll(s, "EXPR", c.expr)
			f.file("Tiltfile", s)

			if c.errorMsg != "" {
				f.loadErrString(c.errorMsg)
				return
			}

			f.load()
			f.assertNextManifest("foo",
				k8sResourceLinks(c.expected),
				db(image("gcr.io/foo")),
				deployment("foo"))
		})

		t.Run("dc-"+c.name, func(t *testing.T) {
			f := newFixture(t)

			f.file("docker-compose.yml", `version: '3.0'
services:
  foo:
    image: gcr.io/foo
`)
			s := `
docker_compose('docker-compose.yml')
dc_resource('foo', links=EXPR)
dc_resource('foo') # test that subsequent calls don't clear the links
`

			s = strings.ReplaceAll(s, "EXPR", c.expr)
			f.file("Tiltfile", s)

			if c.errorMsg != "" {
				f.loadErrString(c.errorMsg)
				return
			}

			f.load()
			f.assertNextManifest("foo",
				dcResourceLinks(c.expected),
			)
		})
	}
}

func TestK8sResourceWithLinksAndPortForwards(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_resource('foo', port_forwards=[8000, 8001], links=link("www.zombo.com", name="zombo"))
`)

	f.load()
	f.assertNextManifest("foo",
		[]model.PortForward{{LocalPort: 8000}, {LocalPort: 8001}},
		k8sResourceLinks{model.MustNewLink("http://www.zombo.com", "zombo")},
		db(image("gcr.io/foo")),
		deployment("foo"))
}

func TestExpand(t *testing.T) {
	f := newFixture(t)
	f.setupExpand()
	f.file("Tiltfile", `
k8s_yaml('all.yaml')
docker_build('gcr.io/a', 'a')
docker_build('gcr.io/b', 'b')
docker_build('gcr.io/c', 'c')
docker_build('gcr.io/d', 'd')
`)
	f.load()
	f.assertNextManifest("a", db(image("gcr.io/a")), deployment("a"))
	f.assertNextManifest("b", db(image("gcr.io/b")), deployment("b"))
	f.assertNextManifest("c", db(image("gcr.io/c")), deployment("c"))
	f.assertNextManifest("d", db(image("gcr.io/d")), deployment("d"))
	f.assertNoMoreManifests() // should be no unresourced yaml remaining
	f.assertConfigFiles("Tiltfile", ".tiltignore", "all.yaml", "a/Dockerfile", "a/.dockerignore", "b/Dockerfile", "b/.dockerignore", "c/Dockerfile", "c/.dockerignore", "d/Dockerfile", "d/.dockerignore")
}

func TestExpandUnresourced(t *testing.T) {
	f := newFixture(t)
	f.dockerfile("a/Dockerfile")

	f.yaml("all.yaml",
		deployment("a", image("gcr.io/a")),
		secret("a-secret"),
	)

	f.gitInit("")
	f.file("Tiltfile", `
k8s_yaml('all.yaml')
docker_build('gcr.io/a', 'a')
`)

	f.load()
	f.assertNextManifest("a", db(image("gcr.io/a")), deployment("a"))
	f.assertNextManifestUnresourced("a-secret")
}

func TestUnresourcedPodCreatorYamlAsManifest(t *testing.T) {
	f := newFixture(t)

	f.yaml("pod_creator.yaml", deployment("pod-creator"), secret("not-pod-creator"))

	f.file("Tiltfile", `
k8s_yaml('pod_creator.yaml')
`)
	f.load()

	f.assertNextManifest("pod-creator", deployment("pod-creator"))
	f.assertNextManifestUnresourced("not-pod-creator")
}

func TestUnresourcedYamlGroupingV1(t *testing.T) {
	f := newFixture(t)

	labelsA := map[string]string{"keyA": "valueA"}
	labelsB := map[string]string{"keyB": "valueB"}
	labelsC := map[string]string{"keyC": "valueC"}
	f.yaml("all.yaml",
		deployment("deployment-a", withLabels(labelsA)),

		deployment("deployment-b", withLabels(labelsB)),
		service("service-b", withLabels(labelsB)),

		deployment("deployment-c", withLabels(labelsC)),
		service("service-c1", withLabels(labelsC)),
		service("service-c2", withLabels(labelsC)),

		secret("someSecret"),
	)

	f.file("Tiltfile", `k8s_yaml('all.yaml')`)
	f.load()

	f.assertNextManifest("deployment-a", deployment("deployment-a"))
	f.assertNextManifest("deployment-b", deployment("deployment-b"), service("service-b"))
	f.assertNextManifest("deployment-c", deployment("deployment-c"), service("service-c1"), service("service-c2"))
	f.assertNextManifestUnresourced("someSecret")
}

func TestUnresourcedYamlGroupingV2(t *testing.T) {
	f := newFixture(t)

	labelsA := map[string]string{"keyA": "valueA"}
	labelsB := map[string]string{"keyB": "valueB"}
	labelsC := map[string]string{"keyC": "valueC"}
	f.yaml("all.yaml",
		deployment("deployment-a", withLabels(labelsA)),

		deployment("deployment-b", withLabels(labelsB)),
		service("service-b", withLabels(labelsB)),

		deployment("deployment-c", withLabels(labelsC)),
		service("service-c1", withLabels(labelsC)),
		service("service-c2", withLabels(labelsC)),

		secret("someSecret"),
	)

	f.file("Tiltfile", `
k8s_yaml('all.yaml')`)
	f.load()

	f.assertNextManifest("deployment-a", deployment("deployment-a"))
	f.assertNextManifest("deployment-b", deployment("deployment-b"), service("service-b"))
	f.assertNextManifest("deployment-c", deployment("deployment-c"), service("service-c1"), service("service-c2"))
	f.assertNextManifestUnresourced("someSecret")
}

func TestK8sGroupedWhenAddedToResource(t *testing.T) {
	f := newFixture(t)
	f.setupExpand()

	labelsA := map[string]string{"keyA": "valueA"}
	labelsB := map[string]string{"keyB": "valueB"}
	labelsC := map[string]string{"keyC": "valueC"}
	f.yaml("all.yaml",
		deployment("deployment-a", image("gcr.io/a"), withLabels(labelsA)),

		deployment("deployment-b", image("gcr.io/b"), withLabels(labelsB)),
		service("service-b", withLabels(labelsB)),

		deployment("deployment-c", image("gcr.io/c"), withLabels(labelsC)),
		service("service-c1", withLabels(labelsC)),
		service("service-c2", withLabels(labelsC)),
	)

	f.file("Tiltfile", `

k8s_yaml('all.yaml')
docker_build('gcr.io/a', 'a')
docker_build('gcr.io/b', 'b')
docker_build('gcr.io/c', 'c')
`)
	f.load()

	f.assertNextManifest("deployment-a", deployment("deployment-a"))
	f.assertNextManifest("deployment-b", deployment("deployment-b"), service("service-b"))
	f.assertNextManifest("deployment-c", deployment("deployment-c"), service("service-c1"), service("service-c2"))
}

func TestImplicitK8sResourceWithoutDockerBuild(t *testing.T) {
	f := newFixture(t)
	f.setupFoo()
	f.file("Tiltfile", `

k8s_yaml('foo.yaml')
k8s_resource('foo', port_forwards=8000)
`)
	f.load()
	f.assertNextManifest("foo", []model.PortForward{{LocalPort: 8000}})
}

func TestExpandTwoDeploymentsWithSameImage(t *testing.T) {
	f := newFixture(t)
	f.setupExpand()
	f.yaml("all.yaml",
		deployment("a", image("gcr.io/a")),
		deployment("a2", image("gcr.io/a")),
		deployment("b", image("gcr.io/b")),
		deployment("c", image("gcr.io/c")),
		deployment("d", image("gcr.io/d")),
	)
	f.file("Tiltfile", `

k8s_yaml('all.yaml')
docker_build('gcr.io/a', 'a')
docker_build('gcr.io/b', 'b')
docker_build('gcr.io/c', 'c')
docker_build('gcr.io/d', 'd')
`)
	f.load()
	f.assertNextManifest("a", db(image("gcr.io/a")), deployment("a"))
	f.assertNextManifest("a2", db(image("gcr.io/a")), deployment("a2"))
	f.assertNextManifest("b", db(image("gcr.io/b")), deployment("b"))
	f.assertNextManifest("c", db(image("gcr.io/c")), deployment("c"))
	f.assertNextManifest("d", db(image("gcr.io/d")), deployment("d"))
}

func TestMultipleYamlFiles(t *testing.T) {
	f := newFixture(t)

	f.setupExpand()
	f.yaml("a.yaml", deployment("a", image("gcr.io/a")))
	f.yaml("b.yaml", deployment("b", image("gcr.io/b")))
	f.yaml("c.yaml", deployment("c", image("gcr.io/c")))
	f.yaml("d.yaml", deployment("d", image("gcr.io/d")))
	f.file("Tiltfile", `
k8s_yaml(['a.yaml', 'b.yaml', 'c.yaml', 'd.yaml'])
docker_build('gcr.io/a', 'a')
docker_build('gcr.io/b', 'b')
docker_build('gcr.io/c', 'c')
docker_build('gcr.io/d', 'd')
`)
	f.load()
	f.assertNextManifest("a", db(image("gcr.io/a")), deployment("a"))
	f.assertNextManifest("b", db(image("gcr.io/b")), deployment("b"))
	f.assertNextManifest("c", db(image("gcr.io/c")), deployment("c"))
	f.assertNextManifest("d", db(image("gcr.io/d")), deployment("d"))
}

func TestLoadOneManifest(t *testing.T) {
	f := newFixture(t)

	f.setupFooAndBar()
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')

docker_build('gcr.io/bar', 'bar')
k8s_yaml('bar.yaml')
`)

	f.load("foo")
	require.Equal(t, []model.ManifestName{"foo"}, f.loadResult.EnabledManifests)

	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "foo.yaml", "bar/Dockerfile", "bar/.dockerignore", "bar.yaml")
}

func TestUncategorizedEnabledEvenIfNotSpecified(t *testing.T) {
	f := newFixture(t)

	f.setupFooAndBar()
	f.yaml("service.yaml", service("some-service"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')

docker_build('gcr.io/bar', 'bar')
k8s_yaml('bar.yaml')

k8s_yaml('service.yaml')
`)

	f.load("foo")
	require.Equal(t, []model.ManifestName{"foo", "uncategorized"}, f.loadResult.EnabledManifests)
}

func TestLoadTypoManifest(t *testing.T) {
	f := newFixture(t)

	f.setupFooAndBar()
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')

docker_build('gcr.io/bar', 'bar')
k8s_yaml('bar.yaml')
`)

	tlr := f.newTiltfileLoader().Load(f.ctx, ctrltiltfile.MainTiltfile(f.JoinPath("Tiltfile"), []string{"baz"}), nil)
	err := tlr.Error
	if assert.Error(t, err) {
		assert.Equal(t, `You specified some resources that could not be found: "baz"
Is this a typo? Existing resources in Tiltfile: "foo", "bar"`, err.Error())
	}
}

func TestBasicGitPathFilter(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("Dockerfile", "FROM golang:1.10")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo', '.')
k8s_yaml('foo.yaml')
`)

	f.load("foo")
	f.assertNextManifest("foo",
		buildFilters(".git"),
		fileChangeFilters(".git"),
		buildFilters("Tiltfile"),
		fileChangeFilters("Tiltfile"),
		buildMatches("foo.yaml"),
		fileChangeMatches("foo.yaml"),
	)
}

func TestCustomBuildGitPathFilter(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("Dockerfile", "FROM golang:1.10")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
custom_build('gcr.io/foo', 'docker build -t gcr.io/foo .', ['.'])
k8s_yaml('foo.yaml')
`)

	f.load("foo")
	f.assertNextManifest("foo",
		fileChangeFilters(".git"),
	)
}

func TestDockerignorePathFilter(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("Dockerfile", "FROM golang:1.10")
	f.file(".dockerignore", "*.txt")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo', '.')
k8s_yaml('foo.yaml')
`)

	f.load("foo")
	f.assertNextManifest("foo",
		buildFilters("a.txt"),
		fileChangeFilters("a.txt"),
		buildMatches("txt.a"),
		fileChangeMatches("txt.a"),
	)
}

// When the custom_build lists one dep, it should pick
// up the dockerignore from that directory.
func TestDockerignoreCustomBuildRelativeDirs(t *testing.T) {
	f := newFixture(t)

	f.file(".dockerignore", "src/sub/a.txt")
	f.file("src/.dockerignore", "sub/b.txt")
	f.file("src/sub/.dockerignore", "c.txt")

	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
custom_build('gcr.io/foo', 'build-image', deps=['./src'])
k8s_yaml('foo.yaml')
`)

	f.load("foo")
	f.assertNextManifest("foo",
		fileChangeFilters("src/sub/b.txt"),
		fileChangeMatches("src/sub/a.txt"),
		fileChangeMatches("src/sub/c.txt"),
	)
}

// When the custom_build lists multiple deps, it should pick
// up the dockerignores from both those directories.
func TestDockerignoreCustomBuildMultipleDeps(t *testing.T) {
	f := newFixture(t)

	f.file(".dockerignore", "src/sub/a.txt")
	f.file("src/.dockerignore", "sub/b.txt")
	f.file("src/sub/.dockerignore", "c.txt")

	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
custom_build('gcr.io/foo', 'build-image', deps=['./src', './src/sub'])
k8s_yaml('foo.yaml')
`)

	f.load("foo")
	f.assertNextManifest("foo",
		fileChangeFilters("src/sub/b.txt"),
		fileChangeMatches("src/sub/a.txt"),
		fileChangeFilters("src/sub/c.txt"),
	)
}

func TestDockerignorePathFilterSubdir(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("foo/Dockerfile", "FROM golang:1.10")
	f.file("foo/.dockerignore", "*.txt")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
`)

	f.load("foo")
	f.assertNextManifest("foo",
		buildFilters("foo/a.txt"),
		fileChangeFilters("foo/a.txt"),
		buildMatches("foo/txt.a"),
		fileChangeMatches("foo/txt.a"),
	)
}

func TestK8sYAMLInputBareString(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.WriteFile("bar.yaml", "im not yaml")
	f.file("Tiltfile", `
k8s_yaml('bar.yaml')
docker_build("gcr.io/foo", "foo", cache='/paths/to/cache')
`)

	f.loadErrString("bar.yaml is not a valid YAML file")
}

func TestK8sYAMLInputFromReadFile(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.file("Tiltfile", `
k8s_yaml(str(read_file('foo.yaml')))
docker_build("gcr.io/foo", "foo", cache='/paths/to/cache')
`)

	if runtime.GOOS == "windows" {
		f.loadErrString("The filename, directory name, or volume label syntax is incorrect")
	} else {
		f.loadErrString("no such file or directory")
	}
}

func TestK8sYAMLInvalid(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.file("Tiltfile", `
k8s_yaml(blob('''apiVersion: v1
kind: Secret
metadata:
  name: mysecret
type: Opaque
data:
  stuff: "!"'''))
docker_build("gcr.io/foo", "foo", cache='/paths/to/cache')
`)

	f.loadErrString(
		`Error reading yaml from Tiltfile blob() call: decoding Secret "mysecret": illegal base64 data at input byte 0`)
}

func TestFilterYamlByLabel(t *testing.T) {
	f := newFixture(t)
	f.file("k8s.yaml", yaml.ConcatYAML(
		testyaml.DoggosDeploymentYaml, testyaml.DoggosServiceYaml,
		testyaml.SnackYaml, testyaml.SanchoYAML))
	f.file("Tiltfile", `
labels = {'app': 'doggos'}
doggos, rest = filter_yaml('k8s.yaml', labels=labels)
k8s_yaml(doggos)
`)

	f.load()
	f.assertNextManifest("doggos", deployment("doggos"), service("doggos"))
	f.assertNoMoreManifests()
}

func TestFilterYamlByName(t *testing.T) {
	f := newFixture(t)
	f.file("k8s.yaml", yaml.ConcatYAML(
		testyaml.DoggosDeploymentYaml, testyaml.DoggosServiceYaml,
		testyaml.SnackYaml, testyaml.SanchoYAML))
	f.file("Tiltfile", `
doggos, rest = filter_yaml('k8s.yaml', name='doggos')
k8s_yaml(doggos)
`)

	f.load()
	f.assertNextManifest("doggos", deployment("doggos"), service("doggos"))
	f.assertNoMoreManifests()
}

func TestFilterYamlByNameKind(t *testing.T) {
	f := newFixture(t)
	f.file("k8s.yaml", yaml.ConcatYAML(
		testyaml.DoggosDeploymentYaml, testyaml.DoggosServiceYaml,
		testyaml.SnackYaml, testyaml.SanchoYAML))
	f.file("Tiltfile", `
doggos, rest = filter_yaml('k8s.yaml', name='doggos', kind='deployment')
k8s_yaml(doggos)
`)

	f.load()
	f.assertNextManifest("doggos", deployment("doggos"))
	f.assertNoMoreManifests()
}

func TestFilterYamlByNamespace(t *testing.T) {
	f := newFixture(t)
	f.file("k8s.yaml", yaml.ConcatYAML(
		testyaml.DoggosDeploymentYaml, testyaml.DoggosServiceYaml,
		testyaml.SnackYaml, testyaml.SanchoYAML))
	f.file("Tiltfile", `
doggos, rest = filter_yaml('k8s.yaml', namespace='the-dog-zone')
k8s_yaml(doggos)
`)

	f.load()
	f.assertNextManifest("doggos", deployment("doggos"))
	f.assertNoMoreManifests()
}

func TestFilterYamlByApiVersion(t *testing.T) {
	f := newFixture(t)
	f.file("k8s.yaml", yaml.ConcatYAML(
		testyaml.DoggosDeploymentYaml, testyaml.DoggosServiceYaml,
		testyaml.SnackYaml, testyaml.SanchoYAML))
	f.file("Tiltfile", `
doggos, rest = filter_yaml('k8s.yaml', name='doggos', api_version='apps/v1')
k8s_yaml(doggos)
`)

	f.load()
	f.assertNextManifest("doggos", deployment("doggos"))
	f.assertNoMoreManifests()
}

func TestFilterYamlNoMatch(t *testing.T) {
	f := newFixture(t)
	f.file("k8s.yaml", yaml.ConcatYAML(testyaml.DoggosDeploymentYaml, testyaml.DoggosServiceYaml))
	f.file("Tiltfile", `
doggos, rest = filter_yaml('k8s.yaml', namespace='dne', kind='deployment')
k8s_yaml(doggos)
`)
	f.loadErrString(emptyYAMLError.Error())
}

func TestYamlNone(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
k8s_yaml(None)
`)
	f.loadErrString(emptyYAMLError.Error())
}

func TestYamlEmptyBlob(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
k8s_yaml(blob(''))
`)
	f.loadErrString(emptyYAMLError.Error())
}

func TestDuplicateLocalResources(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
local_resource('foo', 'echo foo')
local_resource('foo', 'echo foo')
`)

	f.loadErrString(`local_resource named "foo" already exists`)
}

// These tests are for behavior that we specifically enabled in Starlark
// in the init() function
func TestTopLevelIfStatement(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
if True:
  docker_build('gcr.io/foo', 'foo')
  k8s_yaml('foo.yaml')
`)

	f.load()

	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "foo.yaml")
}

func TestTopLevelForLoop(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
for i in range(1, 3):
	print(i)
`)

	f.load()
}

func TestTopLevelVariableRename(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
x = 1
x = 2
`)

	f.load()
}

func TestEmptyDockerfileDockerBuild(t *testing.T) {
	f := newFixture(t)
	f.setupFoo()
	f.file("foo/Dockerfile", "")
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
`)
	f.load()
	m := f.assertNextManifest("foo", db(image("gcr.io/foo")))
	assert.True(t, m.ImageTargetAt(0).IsDockerBuild())
}

func TestSanchoSidecar(t *testing.T) {
	f := newFixture(t)
	f.setupFoo()
	f.file("Dockerfile", "FROM golang:1.10")
	f.file("k8s.yaml", testyaml.SanchoSidecarYAML)
	f.file("Tiltfile", `
k8s_yaml('k8s.yaml')
docker_build('gcr.io/some-project-162817/sancho', '.')
docker_build('gcr.io/some-project-162817/sancho-sidecar', '.')
`)
	f.load()

	assert.Equal(t, 1, len(f.loadResult.Manifests))
	m := f.assertNextManifest("sancho")
	assert.Equal(t, 2, len(m.ImageTargets))
	assert.Equal(t, "gcr.io/some-project-162817/sancho",
		m.ImageTargetAt(0).ImageMapSpec.Selector)
	assert.Equal(t, "gcr.io/some-project-162817/sancho-sidecar",
		m.ImageTargetAt(1).ImageMapSpec.Selector)
}

func TestSanchoRedisSidecar(t *testing.T) {
	f := newFixture(t)
	f.setupFoo()
	f.file("Dockerfile", "FROM golang:1.10")
	f.file("k8s.yaml", testyaml.SanchoRedisSidecarYAML)
	f.file("Tiltfile", `
k8s_yaml('k8s.yaml')
docker_build('gcr.io/some-project-162817/sancho', '.')
`)
	f.load()

	assert.Equal(t, 1, len(f.loadResult.Manifests))
	m := f.assertNextManifest("sancho")
	assert.Equal(t, 1, len(m.ImageTargets))
	assert.Equal(t, "gcr.io/some-project-162817/sancho",
		m.ImageTargetAt(0).ImageMapSpec.Selector)
}

func TestExtraPodSelectors(t *testing.T) {
	f := newFixture(t)

	f.setupExtraPodSelectors("[{'foo': 'bar', 'baz': 'qux'}, {'quux': 'corge'}]")
	f.load()

	f.assertNextManifest("foo",
		extraPodSelectors(labels.Set{"foo": "bar", "baz": "qux"}, labels.Set{"quux": "corge"}),
		podReadiness(model.PodReadinessWait))
}

func TestExtraPodSelectorsNotList(t *testing.T) {
	f := newFixture(t)

	f.setupExtraPodSelectors("'hello'")
	f.loadErrString("got starlark.String", "dict or a list")
}

func TestExtraPodSelectorsDict(t *testing.T) {
	f := newFixture(t)

	f.setupExtraPodSelectors("{'foo': 'bar'}")
	f.load()
	f.assertNextManifest("foo",
		extraPodSelectors(labels.Set{"foo": "bar"}),
		podReadiness(model.PodReadinessWait))
}

func TestExtraPodSelectorsElementNotDict(t *testing.T) {
	f := newFixture(t)

	f.setupExtraPodSelectors("['hello']")
	f.loadErrString("must be dicts", "starlark.String")
}

func TestExtraPodSelectorsKeyNotString(t *testing.T) {
	f := newFixture(t)

	f.setupExtraPodSelectors("[{54321: 'hello'}]")
	f.loadErrString("keys must be strings", "54321")
}

func TestExtraPodSelectorsValueNotString(t *testing.T) {
	f := newFixture(t)

	f.setupExtraPodSelectors("[{'hello': 54321}]")
	f.loadErrString("values must be strings", "54321")
}

func TestPodReadinessDefaultDeployment(t *testing.T) {
	f := newFixture(t)

	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo:stable")))
	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
`)

	f.load("foo")
	f.assertNextManifest("foo",
		deployment("foo"),
		podReadiness(model.PodReadinessWait),
	)
}

func TestPodReadinessDefaultConfigMap(t *testing.T) {
	f := newFixture(t)

	f.file("config.yaml", `apiVersion: v1
kind: ConfigMap
metadata:
  name: config
data:
  foo: bar
`)
	f.file("Tiltfile", `
k8s_yaml('config.yaml')
k8s_resource(new_name='config', objects=['config'])
`)

	f.load("config")
	f.assertNextManifest("config",
		podReadiness(model.PodReadinessIgnore),
	)
}

func TestPodReadinessDefaultJob(t *testing.T) {
	f := newFixture(t)

	f.file("job.yaml", `apiVersion: batch/v1
kind: Job
metadata:
  name: myjob
`)
	f.file("Tiltfile", `
k8s_yaml('job.yaml')
`)

	f.load("myjob")
	f.assertNextManifest("myjob",
		podReadiness(model.PodReadinessSucceeded),
	)
}

func TestK8sDiscoveryStrategy(t *testing.T) {
	f := newFixture(t)

	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo:stable")))
	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
k8s_resource('foo', discovery_strategy='selectors-only')
`)

	f.load("foo")
	f.assertNextManifest("foo",
		deployment("foo"),
		v1alpha1.KubernetesDiscoveryStrategySelectorsOnly,
	)
}

func TestK8sDiscoveryStrategyInvalid(t *testing.T) {
	f := newFixture(t)

	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo:stable")))
	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
k8s_resource('foo', discovery_strategy='typo')
`)

	f.loadErrString("Invalid. Must be one of: \"default\", \"selectors-only\"")
}

func TestPodReadinessOverrideDeployment(t *testing.T) {
	f := newFixture(t)

	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo:stable")))
	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
k8s_resource('foo', pod_readiness='ignore')
`)

	f.load("foo")
	f.assertNextManifest("foo",
		deployment("foo"),
		podReadiness(model.PodReadinessIgnore),
	)
}

func TestPodReadinessOverrideConfigMap(t *testing.T) {
	f := newFixture(t)

	f.file("config.yaml", `apiVersion: v1
kind: ConfigMap
metadata:
  name: config
data:
  foo: "bar"
`)
	f.file("Tiltfile", `
k8s_yaml('config.yaml')
k8s_resource(new_name='config', objects=['config'], pod_readiness='wait')
`)

	f.load("config")
	f.assertNextManifest("config",
		podReadiness(model.PodReadinessWait),
	)
}

func TestPodReadinessInvalid(t *testing.T) {
	f := newFixture(t)

	f.file("config.yaml", `apiVersion: v1
kind: ConfigMap
metadata:
  name: config
data:
  foo: bar
`)
	f.file("Tiltfile", `
k8s_yaml('config.yaml')
k8s_resource(new_name='config', objects=['config'], pod_readiness='w')
`)

	f.loadErrString("Invalid value. Allowed: {ignore, wait}. Got: w")
}

func TestDockerBuildMatchingTag(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("Dockerfile", "FROM golang:1.10")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo:stable")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo:stable', '.')
k8s_yaml('foo.yaml')
`)

	f.load("foo")
	f.assertNextManifest("foo",
		deployment("foo"),
	)
}

func TestDockerBuildButK8sMissing(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("Dockerfile", "FROM golang:1.10")
	f.file("Tiltfile", `
docker_build('gcr.io/foo:stable', '.')
`)

	f.loadAssertWarnings(unmatchedImageNoConfigsWarning)
}

func TestDockerBuildButK8sMissingTag(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("Dockerfile", "FROM golang:1.10")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo:stable', '.')
k8s_yaml('foo.yaml')
`)

	w := unusedImageWarning("gcr.io/foo:stable", []string{"gcr.io/foo"}, "Kubernetes")
	f.loadAssertWarnings(w)
}

func TestDockerBuildUnusedSuppressWarning(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("Dockerfile", "FROM golang:1.10")
	f.file("Tiltfile", `
docker_build('a', '.')
docker_build('b', '.')
update_settings(suppress_unused_image_warnings=['a'])
update_settings(suppress_unused_image_warnings=['b'])
`)

	f.load()
}

func TestDockerBuildButK8sNonMatchingTag(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("Dockerfile", "FROM golang:1.10")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo:beta")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo:stable', '.')
k8s_yaml('foo.yaml')
`)

	w := unusedImageWarning("gcr.io/foo:stable", []string{"gcr.io/foo"}, "Kubernetes")
	f.loadAssertWarnings(w)
}

func TestFail(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
fail("this is an error")
print("not this")
fail("or this")
`)

	f.loadErrString("this is an error")
}

func TestBlob(t *testing.T) {
	f := newFixture(t)

	f.file(
		"Tiltfile",
		fmt.Sprintf(`k8s_yaml(blob('''%s'''))`, testyaml.SnackYaml),
	)

	f.load()

	f.assertNextManifest("snack", deployment("snack"))
}

func TestBlobErr(t *testing.T) {
	f := newFixture(t)

	f.file(
		"Tiltfile",
		`blob(42)`,
	)

	f.loadErrString("for parameter input: got int, want string")
}

func TestImageDependency(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("imageA.dockerfile", "FROM golang:1.10")
	f.file("imageB.dockerfile", "FROM gcr.io/image-a")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/image-b")))
	f.file("Tiltfile", `

docker_build('gcr.io/image-b', '.', dockerfile='imageB.dockerfile')
docker_build('gcr.io/image-a', '.', dockerfile='imageA.dockerfile')
k8s_yaml('foo.yaml')
`)

	f.load()
	f.assertNextManifest("foo", deployment("foo", image("gcr.io/image-a"), image("gcr.io/image-b")))
}

func TestImageDependencyLiveUpdate(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("message.txt", "Hello!")
	f.file("imageA.dockerfile", "FROM golang:1.10")
	f.file("imageB.dockerfile", `FROM gcr.io/image-a
ADD message.txt /tmp/message.txt`)
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/image-b")))
	f.file("Tiltfile", `

docker_build('gcr.io/image-b', '.', dockerfile='imageB.dockerfile',
             live_update=[sync('message.txt', '/tmp/message.txt')])
docker_build('gcr.io/image-a', '.', dockerfile='imageA.dockerfile')
k8s_yaml('foo.yaml')
`)

	f.load()
	m := f.assertNextManifest("foo",
		deployment("foo", image("gcr.io/image-a"), image("gcr.io/image-b")))

	assert.True(t, liveupdate.IsEmptySpec(m.ImageTargetAt(0).LiveUpdateSpec))
	assert.False(t, liveupdate.IsEmptySpec(m.ImageTargetAt(1).LiveUpdateSpec))
}

func TestImageDependencyCycle(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("imageA.dockerfile", "FROM gcr.io/image-b")
	f.file("imageB.dockerfile", "FROM gcr.io/image-a")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/image-b")))
	f.file("Tiltfile", `
docker_build('gcr.io/image-b', '.', dockerfile='imageB.dockerfile')
docker_build('gcr.io/image-a', '.', dockerfile='imageA.dockerfile')
k8s_yaml('foo.yaml')
`)

	f.loadErrString("Image dependency cycle: gcr.io/image-b")
}

func TestImageDependencyDiamond(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("imageA.dockerfile", "FROM golang:1.10")
	f.file("imageB.dockerfile", "FROM gcr.io/image-a")
	f.file("imageC.dockerfile", "FROM gcr.io/image-a")
	f.file("imageD.dockerfile", `
FROM gcr.io/image-b
FROM gcr.io/image-c
`)
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/image-d")))
	f.file("Tiltfile", `

docker_build('gcr.io/image-a', '.', dockerfile='imageA.dockerfile')
docker_build('gcr.io/image-b', '.', dockerfile='imageB.dockerfile')
docker_build('gcr.io/image-c', '.', dockerfile='imageC.dockerfile')
docker_build('gcr.io/image-d', '.', dockerfile='imageD.dockerfile')
k8s_yaml('foo.yaml')
`)

	f.load()

	m := f.assertNextManifest("foo", deployment("foo"))
	assert.Equal(t, []string{
		"gcr.io_image-a",
		"gcr.io_image-b",
		"gcr.io_image-c",
		"gcr.io_image-d",
	}, f.imageTargetNames(m))
}

func TestImageDependencyTwice(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("imageA.dockerfile", "FROM golang:1.10")
	f.file("imageB.dockerfile", `FROM golang:1.10
COPY --from=gcr.io/image-a /src/package.json /src/package.json
COPY --from=gcr.io/image-a /src/package.lock /src/package.lock
`)
	f.file("snack.yaml", `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: snack
  labels:
    app: snack
spec:
  selector:
    matchLabels:
      app: snack
  template:
    metadata:
      labels:
        app: snack
    spec:
      containers:
      - name: snack1
        image: gcr.io/image-b
        command: ["/go/bin/snack"]
      - name: snack2
        image: gcr.io/image-b
        command: ["/go/bin/snack"]
`)
	f.file("Tiltfile", `

docker_build('gcr.io/image-a', '.', dockerfile='imageA.dockerfile')
docker_build('gcr.io/image-b', '.', dockerfile='imageB.dockerfile')
k8s_yaml('snack.yaml')
`)

	f.load()

	m := f.assertNextManifest("snack")
	assert.Equal(t, []string{
		"gcr.io_image-a",
		"gcr.io_image-b",
	}, f.imageTargetNames(m))
	assert.Equal(t, []string{
		"gcr.io_image-a",
		"gcr.io_image-b",
		"snack", // the deploy name
	}, f.idNames(m.DependencyIDs()))
	assert.Equal(t, []string{}, f.idNames(m.ImageTargets[0].DependencyIDs()))
	assert.Equal(t, []string{"gcr.io_image-a"}, f.idNames(m.ImageTargets[1].DependencyIDs()))
	assert.Equal(t, []string{"gcr.io_image-b"}, f.idNames(m.DeployTarget.DependencyIDs()))
}

func TestImageDependencyNormalization(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("common.dockerfile", "FROM golang:1.10")
	f.file("auth.dockerfile", "FROM vandelay/common")
	f.yaml("auth.yaml", deployment("auth", image("vandelay/auth")))
	f.file("Tiltfile", `
docker_build('vandelay/common', '.', dockerfile='common.dockerfile')
docker_build('vandelay/auth', '.', dockerfile='auth.dockerfile')
k8s_yaml('auth.yaml')
`)

	f.load()

	m := f.assertNextManifest("auth", deployment("auth"))
	assert.Equal(t, []string{
		"vandelay_common",
		"vandelay_auth",
	}, f.imageTargetNames(m))
}

func TestImagesWithSameNameAssembly(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("app.dockerfile", "FROM golang:1.10")
	f.file("app-jessie.dockerfile", "FROM golang:1.10-jessie")
	f.yaml("app.yaml",
		deployment("app", image("vandelay/app")),
		deployment("app-jessie", image("vandelay/app:jessie")))
	f.file("Tiltfile", `

docker_build('vandelay/app', '.', dockerfile='app.dockerfile')
docker_build('vandelay/app:jessie', '.', dockerfile='app-jessie.dockerfile')
k8s_yaml('app.yaml')
`)

	f.load()

	f.assertNextManifest("app", deployment("app", image("vandelay/app")))
	f.assertNextManifest("app-jessie", deployment("app-jessie", image("vandelay/app:jessie")))
}

func TestImagesWithSameNameDifferentManifests(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("app.dockerfile", "FROM golang:1.10")
	f.file("app-jessie.dockerfile", "FROM golang:1.10-jessie")
	f.yaml("app.yaml",
		deployment("app", image("vandelay/app")),
		deployment("app-jessie", image("vandelay/app:jessie")))
	f.file("Tiltfile", `
docker_build('vandelay/app', '.', dockerfile='app.dockerfile')
docker_build('vandelay/app:jessie', '.', dockerfile='app-jessie.dockerfile')
k8s_yaml('app.yaml')
`)

	f.load()

	m := f.assertNextManifest("app", deployment("app"))
	assert.Equal(t, []string{
		"vandelay_app",
	}, f.imageTargetNames(m))

	m = f.assertNextManifest("app-jessie", deployment("app-jessie"))
	assert.Equal(t, []string{
		"vandelay_app:jessie",
	}, f.imageTargetNames(m))
}

func TestImageRefSuggestion(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.file("Tiltfile", `
docker_build('gcr.typo.io/foo', 'foo')
k8s_yaml('foo.yaml')
`)

	w := unusedImageWarning("gcr.typo.io/foo", []string{"gcr.io/foo"}, "Kubernetes")
	f.loadAssertWarnings(w)
}

func TestDir(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.yaml("config/foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.yaml("config/bar.yaml", deployment("bar", image("gcr.io/bar")))
	f.file("Tiltfile", `k8s_yaml(listdir('config'))`)

	f.load("foo", "bar")
	f.assertNumManifests(2)
	f.assertConfigFiles("Tiltfile", ".tiltignore", "config/foo.yaml", "config/bar.yaml")
}

func TestDirRecursive(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("foo/bar", "bar")
	f.file("foo/baz/qux", "qux")
	f.file("Tiltfile", `files = listdir('foo', recursive=True)

for f in files:
  read_file(f)
`)

	f.load()
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo", "foo/bar", "foo/baz/qux")
}

func TestCallCounts(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("Dockerfile", "FROM golang:1.10")
	f.yaml("foo.yaml",
		deployment("foo", image("gcr.io/foo")),
		deployment("bar", image("gcr.io/bar")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo', '.')
docker_build('gcr.io/bar', '.')
k8s_yaml('foo.yaml')
`)

	f.load()

	require.Len(t, f.an.Counts, 1)
	expectedCallCounts := map[string]int{
		"docker_build": 2,
		"k8s_yaml":     1,
	}
	tags := f.an.Counts[0].Tags
	for arg, expectedCount := range expectedCallCounts {
		count, ok := tags[fmt.Sprintf("tiltfile.invoked.%s", arg)]
		require.True(t, ok, "arg %s was not counted in %v", arg, tags)
		require.Equal(t, strconv.Itoa(expectedCount), count, "arg %s had the wrong count in %v", arg, tags)
	}
}

func TestArgCounts(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("Dockerfile", "FROM golang:1.10")
	f.yaml("foo.yaml",
		deployment("foo", image("gcr.io/foo")),
		deployment("bar", image("gcr.io/bar")))
	f.file("Tiltfile", `
docker_build(ref='gcr.io/foo', context='.', dockerfile='Dockerfile')
docker_build('gcr.io/bar', '.')
k8s_yaml('foo.yaml')
`)

	f.load()

	require.Len(t, f.an.Counts, 1)
	expectedArgCounts := map[string]int{
		"docker_build.arg.context":    2,
		"docker_build.arg.dockerfile": 1,
		"docker_build.arg.ref":        2,
		"k8s_yaml.arg.yaml":           1,
	}
	tags := f.an.Counts[0].Tags
	for arg, expectedCount := range expectedArgCounts {
		count, ok := tags[fmt.Sprintf("tiltfile.invoked.%s", arg)]
		require.True(t, ok, "tiltfile.invoked.%s was not counted in %v", arg, tags)
		require.Equal(t, strconv.Itoa(expectedCount), count, "tiltfile.invoked.%s had the wrong count in %v", arg, tags)
	}
}

func TestK8sManifestRefInjectCounts(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("Dockerfile", "FROM golang:1.10")
	f.file("sancho_twin.yaml", testyaml.SanchoTwoContainersOneImageYAML) // 1 img x 2 c
	f.file("sancho_sidecar.yaml", testyaml.SanchoSidecarYAML)            // 2 imgs (1 c each)
	f.file("blorg.yaml", testyaml.BlorgJobYAML)

	f.file("Tiltfile", `
docker_build('gcr.io/some-project-162817/sancho', '.')
docker_build('gcr.io/some-project-162817/sancho-sidecar', '.')
docker_build('gcr.io/blorg-dev/blorg-backend:devel-nick', '.')

k8s_yaml(['sancho_twin.yaml', 'sancho_sidecar.yaml', 'blorg.yaml'])
`)

	f.load()

	sanchoTwin := f.assertNextManifest("sancho-2c1i")
	sTwinInjectCounts := sanchoTwin.K8sTarget().RefInjectCounts()
	assert.Len(t, sTwinInjectCounts, 1)
	assert.Equal(t, sTwinInjectCounts[testyaml.SanchoImage], 2)

	sanchoSidecar := f.assertNextManifest("sancho")
	ssInjectCounts := sanchoSidecar.K8sTarget().RefInjectCounts()
	assert.Len(t, ssInjectCounts, 2)
	assert.Equal(t, ssInjectCounts[testyaml.SanchoImage], 1)
	assert.Equal(t, ssInjectCounts[testyaml.SanchoSidecarImage], 1)

	blorgJob := f.assertNextManifest("blorg-job")
	blorgInjectCounts := blorgJob.K8sTarget().RefInjectCounts()
	assert.Len(t, blorgInjectCounts, 1)
	assert.Equal(t, blorgJob.K8sTarget().RefInjectCounts()["gcr.io/blorg-dev/blorg-backend:devel-nick"], 1)
}

func TestYamlErrorFromLocal(t *testing.T) {
	f := newFixture(t)
	f.file("Tiltfile", `
yaml = local('echo hi')
k8s_yaml(yaml)
`)
	f.loadErrString("echo hi")
}

func TestYamlErrorFromReadFile(t *testing.T) {
	f := newFixture(t)
	f.file("foo.yaml", "hi")
	f.file("Tiltfile", `
k8s_yaml(read_file('foo.yaml'))
`)
	f.loadErrString(fmt.Sprintf("file: %s", f.JoinPath("foo.yaml")))
}

func TestYamlErrorFromBlob(t *testing.T) {
	f := newFixture(t)
	f.file("Tiltfile", `
k8s_yaml(blob('hi'))
`)
	f.loadErrString("from Tiltfile blob() call")
}

func TestCustomBuildWithTag(t *testing.T) {
	f := newFixture(t)

	tiltfile := `k8s_yaml('foo.yaml')
custom_build(
  'gcr.io/foo',
  'docker build -t gcr.io/foo:my-great-tag foo',
  ['foo'],
  tag='my-great-tag'
)`

	f.setupFoo()
	f.file("Tiltfile", tiltfile)

	f.load("foo")
	f.assertNumManifests(1)
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo.yaml", "foo/.dockerignore")
	m := f.assertNextManifest("foo",
		cb(
			image("gcr.io/foo"),
			deps(f.JoinPath("foo")),
			cmd("docker build -t gcr.io/foo:my-great-tag foo", f.Path()),
			tag("my-great-tag"),
		),
		deployment("foo"))
	assert.False(t, m.ImageTargets[0].CustomBuildInfo().SkipsPush())
}

func TestCustomBuildDisablePush(t *testing.T) {
	f := newFixture(t)

	tiltfile := `k8s_yaml('foo.yaml')
hfb = custom_build(
  'gcr.io/foo',
  'docker build -t $TAG foo',
	['foo'],
	disable_push=True,
)`

	f.setupFoo()
	f.file("Tiltfile", tiltfile)

	f.load("foo")
	f.assertNumManifests(1)
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo.yaml", "foo/.dockerignore")
	f.assertNextManifest("foo",
		cb(
			image("gcr.io/foo"),
			deps(f.JoinPath("foo")),
			cmd("docker build -t $TAG foo", f.Path()),
			disablePush(true),
		),
		deployment("foo"))
}

func TestCustomBuildSkipsLocalDocker(t *testing.T) {
	f := newFixture(t)

	tiltfile := `
k8s_yaml('foo.yaml')
custom_build(
  'gcr.io/foo',
  'buildah bud -t $TAG foo && buildah push $TAG $TAG',
	['foo'],
	skips_local_docker=True,
)`

	f.setupFoo()
	f.file("Tiltfile", tiltfile)

	f.load("foo")
	m := f.assertNextManifest("foo",
		cb(
			image("gcr.io/foo"),
		),
		deployment("foo"))
	assert.Equal(t, v1alpha1.CmdImageOutputRemote, m.ImageTargets[0].CustomBuildInfo().OutputMode)
	assert.True(t, m.ImageTargets[0].CustomBuildInfo().SkipsPush())
}

func TestImageObjectJSONPath(t *testing.T) {
	f := newFixture(t)
	f.file("um.yaml", `apiVersion: tilt.dev/v1alpha1
kind: UselessMachine
metadata:
  name: um
spec:
  image:
    repo: tilt.dev/frontend`)
	f.dockerfile("Dockerfile")
	f.file("Tiltfile", `
k8s_yaml('um.yaml')
k8s_kind(kind='UselessMachine', image_object={'json_path': '{.spec.image}', 'repo_field': 'repo', 'tag_field': 'tag'})
docker_build('tilt.dev/frontend', '.')
`)

	f.load()
	m := f.assertNextManifest("um",
		podReadiness(model.PodReadinessWait))
	assert.Equal(t, "tilt.dev/frontend",
		m.ImageTargets[0].ImageMapSpec.Selector)
}

func TestImageObjectJSONPathNoMatch(t *testing.T) {
	f := newFixture(t)
	f.file("um.yaml", `apiVersion: tilt.dev/v1alpha1
kind: UselessMachine
metadata:
  name: um
spec:
  repo: tilt.dev/frontend`)
	f.dockerfile("Dockerfile")
	f.file("Tiltfile", `
k8s_yaml('um.yaml')
k8s_kind(kind='UselessMachine', image_object={'json_path': '{.spec.image}', 'repo_field': 'repo', 'tag_field': 'tag'})
docker_build('tilt.dev/frontend', '.')
`)

	f.loadErrString("finding image", "UselessMachine/um", ".spec.image")
}

func TestImageObjectJSONPathPodReadinessIgnore(t *testing.T) {
	f := newFixture(t)
	f.file("um.yaml", `apiVersion: tilt.dev/v1alpha1
kind: UselessMachine
metadata:
  name: um
spec:
  image:
    repo: tilt.dev/frontend`)
	f.dockerfile("Dockerfile")
	f.file("Tiltfile", `
k8s_yaml('um.yaml')
k8s_kind(kind='UselessMachine', pod_readiness='ignore',
         image_object={'json_path': '{.spec.image}', 'repo_field': 'repo', 'tag_field': 'tag'})
docker_build('tilt.dev/frontend', '.')
`)

	f.load()
	m := f.assertNextManifest("um",
		podReadiness(model.PodReadinessIgnore))
	assert.Equal(t, "tilt.dev/frontend",
		m.ImageTargets[0].ImageMapSpec.Selector)
}

func TestExtraImageLocationOneImage(t *testing.T) {
	f := newFixture(t)
	f.setupCRD()
	f.dockerfile("env/Dockerfile")
	f.dockerfile("builder/Dockerfile")
	f.file("Tiltfile", `

k8s_yaml('crd.yaml')
k8s_image_json_path(kind='Environment', paths='{.spec.runtime.image}')
docker_build('test/mycrd-env', 'env')
`)

	f.load("mycrd")
	f.assertNextManifest("mycrd",
		db(
			image("test/mycrd-env"),
		),
		k8sObject("mycrd", "Environment"),
	)
}

func TestConflictingWorkloadNames(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("foo1/Dockerfile")
	f.dockerfile("foo2/Dockerfile")
	f.yaml("foo1.yaml", deployment("foo", image("gcr.io/foo1"), namespace("ns1")))
	f.yaml("foo2.yaml", deployment("foo", image("gcr.io/foo2"), namespace("ns2")))

	f.file("Tiltfile", `

k8s_yaml(['foo1.yaml', 'foo2.yaml'])
docker_build('gcr.io/foo1', 'foo1')
docker_build('gcr.io/foo2', 'foo2')
`)
	f.load("foo:deployment:ns1", "foo:deployment:ns2")

	f.assertNextManifest("foo:deployment:ns1", db(image("gcr.io/foo1")))
	f.assertNextManifest("foo:deployment:ns2", db(image("gcr.io/foo2")))
}

type k8sKindTest struct {
	name                 string
	k8sKindArgs          string
	expectWorkload       bool
	expectImage          bool
	expectedError        string
	preamble             string
	expectedResourceName model.ManifestName
}

func TestK8sKind(t *testing.T) {
	tests := []k8sKindTest{
		{name: "match kind", k8sKindArgs: "'Environment', image_json_path='{.spec.runtime.image}'", expectWorkload: true, expectImage: true},
		{name: "don't match kind", k8sKindArgs: "'fdas', image_json_path='{.spec.runtime.image}'", expectWorkload: false},
		{name: "match apiVersion", k8sKindArgs: "'Environment', image_json_path='{.spec.runtime.image}', api_version='fission.io/v1'", expectWorkload: true, expectImage: true},
		{name: "don't match apiVersion", k8sKindArgs: "'Environment', image_json_path='{.spec.runtime.image}', api_version='fission.io/v2'"},
		{name: "invalid kind regexp", k8sKindArgs: "'*', image_json_path='{.spec.runtime.image}'", expectedError: "error parsing kind regexp"},
		{name: "invalid apiVersion regexp", k8sKindArgs: "'Environment', api_version='*', image_json_path='{.spec.runtime.image}'", expectedError: "error parsing apiVersion regexp"},
		{name: "no image", k8sKindArgs: "'Environment'", expectWorkload: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			f.setupCRD()
			f.dockerfile("env/Dockerfile")
			f.dockerfile("builder/Dockerfile")
			img := ""
			if !test.expectWorkload || test.expectImage {
				img = "docker_build('test/mycrd-env', 'env')"
			}
			f.file("Tiltfile", fmt.Sprintf(`

%s
k8s_yaml('crd.yaml')
k8s_kind(%s)
%s
`, test.preamble, test.k8sKindArgs, img))

			if test.expectWorkload {
				if test.expectedError != "" {
					t.Fatal("invalid test: cannot expect both workload and error")
				}
				expectedResourceName := model.ManifestName("mycrd")
				if test.expectedResourceName != "" {
					expectedResourceName = test.expectedResourceName
				}
				f.load(string(expectedResourceName))
				var imageOpt interface{}
				if test.expectImage {
					imageOpt = db(image("test/mycrd-env"))
				} else {
					imageOpt = funcOpt(func(t *testing.T, m model.Manifest) bool {
						return assert.Equal(t, 0, len(m.ImageTargets))
					})
				}
				f.assertNextManifest(
					expectedResourceName,
					k8sObject("mycrd", "Environment"),
					imageOpt)
			} else {
				if test.expectImage {
					t.Fatal("invalid test: cannot expect image without expecting workload")
				}
				if test.expectedError == "" {
					f.loadAssertWarnings(unmatchedImageAllUnresourcedWarning)
				} else {
					f.loadErrString(test.expectedError)
				}
			}
		})
	}
}

func TestK8sKindImageJSONPathPositional(t *testing.T) {
	f := newFixture(t)
	f.setupCRD()
	f.dockerfile("env/Dockerfile")
	f.dockerfile("builder/Dockerfile")
	f.file("Tiltfile", `k8s_yaml('crd.yaml')
k8s_kind('Environment', '{.spec.runtime.image}')
docker_build('test/mycrd-env', 'env')
`)

	f.loadErrString("got 2 arguments, want at most 1")
}

func TestExtraImageLocationTwoImages(t *testing.T) {
	f := newFixture(t)
	f.setupCRD()
	f.dockerfile("env/Dockerfile")
	f.dockerfile("builder/Dockerfile")
	f.file("Tiltfile", `

k8s_yaml('crd.yaml')
k8s_image_json_path(['{.spec.runtime.image}', '{.spec.builder.image}'], kind='Environment')
docker_build('test/mycrd-builder', 'builder')
docker_build('test/mycrd-env', 'env')
`)

	f.load("mycrd")
	f.assertNextManifest("mycrd",
		db(
			image("test/mycrd-env"),
		),
		db(
			image("test/mycrd-builder"),
		),
		k8sObject("mycrd", "Environment"),
	)
}

func TestExtraImageLocationDeploymentEnvVarByName(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("foo/Dockerfile")
	f.dockerfile("foo-fetcher/Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo"), withEnvVars("FETCHER_IMAGE", "gcr.io/foo-fetcher")))
	f.dockerfile("bar/Dockerfile")
	// just throwing bar in here to make sure it doesn't error out because it has no FETCHER_IMAGE
	f.yaml("bar.yaml", deployment("bar", image("gcr.io/bar")))
	f.gitInit("")

	f.file("Tiltfile", `k8s_yaml(['foo.yaml', 'bar.yaml'])
docker_build('gcr.io/foo', 'foo')
docker_build('gcr.io/foo-fetcher', 'foo-fetcher')
docker_build('gcr.io/bar', 'bar')
k8s_image_json_path("{.spec.template.spec.containers[*].env[?(@.name=='FETCHER_IMAGE')].value}", name='foo')
	`)
	f.load("foo", "bar")
	f.assertNextManifest("foo",
		db(
			image("gcr.io/foo"),
		),
		db(
			image("gcr.io/foo-fetcher"),
		),
	)
	f.assertNextManifest("bar",
		db(
			image("gcr.io/bar"),
		),
	)
}

func TestExtraImageLocationDeploymentEnvVarMatch(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("foo/Dockerfile")
	f.dockerfile("foo-fetcher/Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo"), withEnvVars("FETCHER_IMAGE", "gcr.io/foo-fetcher")))
	f.gitInit("")

	f.file("Tiltfile", `k8s_yaml('foo.yaml')
docker_build('gcr.io/foo', 'foo')
docker_build('gcr.io/foo-fetcher', 'foo-fetcher', match_in_env_vars=True)
	`)
	f.load("foo")
	f.assertNextManifest("foo",
		db(
			image("gcr.io/foo"),
		),
		db(
			image("gcr.io/foo-fetcher").withMatchInEnvVars(),
		),
	)
}

func TestExtraImageLocationDeploymentEnvVarDoesNotMatchIfNotSpecified(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("foo/Dockerfile")
	f.dockerfile("foo-fetcher/Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo"), withEnvVars("FETCHER_IMAGE", "gcr.io/foo-fetcher")))
	f.gitInit("")

	f.file("Tiltfile", `k8s_yaml('foo.yaml')
docker_build('gcr.io/foo', 'foo')
docker_build('gcr.io/foo-fetcher', 'foo-fetcher')
	`)
	f.loadAssertWarnings(unusedImageWarning("gcr.io/foo-fetcher", []string{"gcr.io/foo"}, "Kubernetes"))
	f.assertNextManifest("foo",
		db(
			image("gcr.io/foo"),
		),
	)

}

func TestK8sImageJSONPathArgs(t *testing.T) {
	tests := []struct {
		name          string
		args          string
		expectMatch   bool
		expectedError string
	}{
		{"match name", "name='foo'", true, ""},
		{"don't match name", "name='bar'", false, ""},
		{"match name w/ regex", "name='.*o'", true, ""},
		{"match kind", "name='foo', kind='Deployment'", true, ""},
		{"don't match kind", "name='bar', kind='asdf'", false, ""},
		{"match apiVersion", "name='foo', api_version='apps/v1'", true, ""},
		{"match apiVersion+kind w/ regex", "name='foo', kind='Deployment', api_version='apps/.*'", true, ""},
		{"don't match apiVersion", "name='bar', api_version='apps/v2'", false, ""},
		{"match namespace", "name='foo', namespace='default'", true, ""},
		{"don't match namespace", "name='bar', namespace='asdf'", false, ""},
		{"invalid name regex", "name='*'", false, "error parsing name regexp"},
		{"invalid kind regex", "kind='*'", false, "error parsing kind regexp"},
		{"invalid apiVersion regex", "name='foo', api_version='*'", false, "error parsing apiVersion regexp"},
		{"invalid namespace regex", "namespace='*'", false, "error parsing namespace regexp"},
		{"regexes are case-insensitive", "name='FOO'", true, ""},
		{"regexes that specify case insensitivity still work", "name='(?i)FOO'", true, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)

			f.dockerfile("foo/Dockerfile")
			f.dockerfile("foo-fetcher/Dockerfile")
			f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo"), withEnvVars("FETCHER_IMAGE", "gcr.io/foo-fetcher")))
			f.gitInit("")

			f.file("Tiltfile", fmt.Sprintf(`k8s_yaml('foo.yaml')
docker_build('gcr.io/foo', 'foo')
docker_build('gcr.io/foo-fetcher', 'foo-fetcher')
k8s_image_json_path("{.spec.template.spec.containers[*].env[?(@.name=='FETCHER_IMAGE')].value}", %s)
	`, test.args))
			if test.expectMatch {
				if test.expectedError != "" {
					t.Fatal("illegal test definition: cannot expect both match and error")
				}
				f.load("foo")
				f.assertNextManifest("foo",
					db(
						image("gcr.io/foo"),
					),
					db(
						image("gcr.io/foo-fetcher"),
					),
				)
			} else {
				if test.expectedError == "" {
					w := unusedImageWarning("gcr.io/foo-fetcher", []string{"gcr.io/foo"}, "Kubernetes")
					f.loadAssertWarnings(w)
				} else {
					f.loadErrString(test.expectedError)
				}
			}
		})
	}
}

func TestExtraImageLocationDeploymentEnvVarByNameAndNamespace(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("foo/Dockerfile")
	f.dockerfile("foo-fetcher/Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo"), withEnvVars("FETCHER_IMAGE", "gcr.io/foo-fetcher")))
	f.gitInit("")

	f.file("Tiltfile", `k8s_yaml('foo.yaml')
docker_build('gcr.io/foo', 'foo')
docker_build('gcr.io/foo-fetcher', 'foo-fetcher')
k8s_image_json_path("{.spec.template.spec.containers[*].env[?(@.name=='FETCHER_IMAGE')].value}", name='foo', namespace='default')
	`)
	f.load("foo")
	f.assertNextManifest("foo",
		db(
			image("gcr.io/foo"),
		),
		db(
			image("gcr.io/foo-fetcher"),
		),
	)
}

func TestExtraImageLocationNoMatch(t *testing.T) {
	f := newFixture(t)
	f.setupCRD()
	f.dockerfile("env/Dockerfile")
	f.dockerfile("builder/Dockerfile")
	f.file("Tiltfile", `k8s_yaml('crd.yaml')
k8s_image_json_path('{.foobar}', kind='Environment')
docker_build('test/mycrd-env', 'env')
`)

	f.loadErrString("{.foobar}", "foobar is not found")
}

func TestExtraImageLocationInvalidJsonPath(t *testing.T) {
	f := newFixture(t)
	f.setupCRD()
	f.dockerfile("env/Dockerfile")
	f.dockerfile("builder/Dockerfile")
	f.file("Tiltfile", `k8s_yaml('crd.yaml')
k8s_image_json_path('{foobar()}', kind='Environment')
docker_build('test/mycrd-env', 'env')
`)

	f.loadErrString("{foobar()}", "unrecognized identifier foobar()")
}

func TestExtraImageLocationNoPaths(t *testing.T) {
	f := newFixture(t)
	f.file("Tiltfile", `k8s_image_json_path(kind='MyType')`)
	f.loadErrString("missing argument for paths")
}

func TestExtraImageLocationNotListOrString(t *testing.T) {
	f := newFixture(t)
	f.file("Tiltfile", `k8s_image_json_path(kind='MyType', paths=8)`)
	f.loadErrString("for parameter \"paths\": Expected string, got: 8")
}

func TestExtraImageLocationListContainsNonString(t *testing.T) {
	f := newFixture(t)
	f.file("Tiltfile", `k8s_image_json_path(kind='MyType', paths=["foo", 8])`)
	f.loadErrString("for parameter \"paths\": Expected string, got: 8")
}

func TestExtraImageLocationNoSelectorSpecified(t *testing.T) {
	f := newFixture(t)
	f.file("Tiltfile", `k8s_image_json_path(paths=["foo"])`)
	f.loadErrString("at least one of kind, name, or namespace must be specified")
}

func TestDockerBuildEmptyDockerFileArg(t *testing.T) {
	f := newFixture(t)
	f.file("Tiltfile", `
docker_build('web/api', '', dockerfile='')
`)
	f.loadErrString("error reading dockerfile")
}

func TestK8sYamlEmptyArg(t *testing.T) {
	f := newFixture(t)
	f.file("Tiltfile", `
k8s_yaml('')
`)
	f.loadErrString("error reading yaml file")
}

func TestTwoDefaultRegistries(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
default_registry("gcr.io")
default_registry("docker.io")`)

	f.loadErrString("default registry already defined")
}

func TestDefaultRegistryInvalid(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.file("Tiltfile", `
default_registry("foo")
docker_build('gcr.io/foo', 'foo')
`)

	f.loadErrString("Traceback ", "repository name must be canonical")
}

func TestDefaultRegistryHostFromCluster(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.file("Tiltfile", `
default_registry("abc.io", host_from_cluster="def.io")
k8s_yaml('foo.yaml')
docker_build('gcr.io/foo', 'foo')
`)

	f.load()

	f.assertNextManifest("foo",
		db(image("gcr.io/foo").withLocalRef("abc.io/gcr.io_foo").withClusterRef("def.io/gcr.io_foo")),
		deployment("foo"))
}

func TestDefaultRegistryAtEndOfTiltfile(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	// default_registry is the last entry to test that it doesn't only affect subsequently defined images
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
default_registry('bar.com')
`)

	f.load()

	f.assertNextManifest("foo",
		db(image("gcr.io/foo").withLocalRef("bar.com/gcr.io_foo")),
		deployment("foo"))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "foo.yaml")
}

func TestDefaultRegistryTwoImagesOnlyDifferByTag(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("bar/Dockerfile")
	f.yaml("bar.yaml", deployment("bar", image("gcr.io/foo:bar")))

	f.dockerfile("baz/Dockerfile")
	f.yaml("baz.yaml", deployment("baz", image("gcr.io/foo:baz")))

	f.gitInit("")
	f.file("Tiltfile", `

docker_build('gcr.io/foo:bar', 'bar')
docker_build('gcr.io/foo:baz', 'baz')
k8s_yaml('bar.yaml')
k8s_yaml('baz.yaml')
default_registry('example.com')
`)

	f.load()

	f.assertNextManifest("bar",
		db(image("gcr.io/foo:bar").withLocalRef("example.com/gcr.io_foo")),
		deployment("bar"))
	f.assertNextManifest("baz",
		db(image("gcr.io/foo:baz").withLocalRef("example.com/gcr.io_foo")),
		deployment("baz"))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "bar/Dockerfile", "bar/.dockerignore", "bar.yaml", "baz/Dockerfile", "baz/.dockerignore", "baz.yaml")
}

func TestDefaultRegistrySingleName(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("fe/Dockerfile")
	f.yaml("fe.yaml", deployment("fe", image("fe")))

	f.dockerfile("be/Dockerfile")
	f.yaml("be.yaml", deployment("be", image("be")))

	f.gitInit("")
	f.file("Tiltfile", `

docker_build('fe', './fe')
docker_build('be', './be')
k8s_yaml('fe.yaml')
k8s_yaml('be.yaml')
default_registry('123.dkr.ecr.us-east-1.amazonaws.com', single_name='team-a/dev')
`)

	f.load()

	fe := f.assertNextManifest("fe",
		db(image("fe").withLocalRef("123.dkr.ecr.us-east-1.amazonaws.com/team-a/dev")),
		deployment("fe"))

	feRefs, err := fe.ImageTargets[0].Refs(f.cluster(fe))
	assert.NoError(t, err)
	feTaggedRefs, err := feRefs.AddTagSuffix("tilt-build-123")
	assert.NoError(t, err)
	assert.Equal(t, "123.dkr.ecr.us-east-1.amazonaws.com/team-a/dev:fe-tilt-build-123",
		feTaggedRefs.LocalRef.String())

	be := f.assertNextManifest("be",
		db(image("be").withLocalRef("123.dkr.ecr.us-east-1.amazonaws.com/team-a/dev")),
		deployment("be"))

	beRefs, err := be.ImageTargets[0].Refs(f.cluster(be))
	assert.NoError(t, err)
	beTaggedRefs, err := beRefs.AddTagSuffix("tilt-build-456")
	assert.NoError(t, err)
	assert.Equal(t, "123.dkr.ecr.us-east-1.amazonaws.com/team-a/dev:be-tilt-build-456",
		beTaggedRefs.LocalRef.String())
}

func TestDefaultReadFile(t *testing.T) {
	f := newFixture(t)
	f.setupFooAndBar()
	tiltfile := `
result = read_file("this_file_does_not_exist", default="foo")
docker_build('gcr.io/foo', 'foo')
k8s_yaml(str(result) + '.yaml')
`

	f.file("Tiltfile", tiltfile)

	f.load()

	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))

	f.assertConfigFiles("Tiltfile", ".tiltignore", "this_file_does_not_exist", "foo.yaml", "foo/Dockerfile", "foo/.dockerignore")
}

func TestWatchFile(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("hello", "world")
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
watch_file('hello')
k8s_yaml('foo.yaml')
`)

	f.load()

	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "foo.yaml", "hello")
}

func TestAssemblyBasic(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
`)

	f.load("foo")

	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))

	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo.yaml", "foo/Dockerfile", "foo/.dockerignore")
}

func TestAssemblyTwoWorkloadsSameImage(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("bar.yaml", deployment("bar", image("gcr.io/foo")))

	f.file("Tiltfile", `

docker_build('gcr.io/foo', 'foo')
k8s_yaml(['foo.yaml', 'bar.yaml'])
`)

	f.load("foo", "bar")

	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))
	f.assertNextManifest("bar",
		db(image("gcr.io/foo")),
		deployment("bar"))

	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo.yaml", "bar.yaml", "foo/Dockerfile", "foo/.dockerignore")
}

// Fix a bug where a service with no selectors trivially matched all pods, so Tilt grouped
// it with the first workload (https://github.com/tilt-dev/tilt/issues/4233)
func TestAssemblyServiceWithoutSelectorMatchesNothing(t *testing.T) {
	f := newFixture(t)

	f.yaml("all.yaml",
		deployment("foo", withLabels(map[string]string{"app": "foo"})),

		service("service-without-selectors", withLabels(map[string]string{})),
	)
	f.file("Tiltfile", `
k8s_yaml('all.yaml')
`)

	f.load()

	f.assertNextManifest("foo", deployment("foo"))

	f.assertNextManifestUnresourced("service-without-selectors")
}

func TestK8sResourceNoMatch(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.file("Tiltfile", `

k8s_yaml('foo.yaml')
k8s_resource('bar', new_name='baz')
`)

	f.loadErrString("specified unknown resource \"bar\". known k8s resources: foo")
}

func TestK8sResourceNewName(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.file("Tiltfile", `

k8s_yaml('foo.yaml')
k8s_resource('foo', new_name='bar')
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("bar", deployment("foo"))
}

func TestK8sResourceRenameTwice(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
k8s_resource('foo', new_name='bar')
k8s_resource('bar', new_name='baz')
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("baz", deployment("foo"))
}

func TestK8sResourceNewNameConflict(t *testing.T) {
	f := newFixture(t)

	f.setupFooAndBar()
	f.file("Tiltfile", `

k8s_yaml(['foo.yaml', 'bar.yaml'])
k8s_resource('foo', new_name='bar')
`)

	f.loadErrString("\"foo\" to \"bar\"", "already exists")
}

func TestK8sResourceRenameConflictingNames(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("foo1/Dockerfile")
	f.dockerfile("foo2/Dockerfile")
	f.yaml("foo1.yaml", deployment("foo", image("gcr.io/foo1"), namespace("ns1")))
	f.yaml("foo2.yaml", deployment("foo", image("gcr.io/foo2"), namespace("ns2")))

	f.file("Tiltfile", `

k8s_yaml(['foo1.yaml', 'foo2.yaml'])
docker_build('gcr.io/foo1', 'foo1')
docker_build('gcr.io/foo2', 'foo2')
k8s_resource('foo:deployment:ns2', new_name='foo')
`)
	f.load("foo:deployment:ns1", "foo")

	f.assertNextManifest("foo:deployment:ns1", db(image("gcr.io/foo1")))
	f.assertNextManifest("foo", db(image("gcr.io/foo2")))
}

func TestConflictingNewNames(t *testing.T) {
	f := newFixture(t)

	f.yaml("ns1.yaml", namespace("ns1"))
	f.yaml("ns2.yaml", namespace("ns2"))
	f.file("Tiltfile", `
k8s_yaml(['ns1.yaml', 'ns2.yaml'])
k8s_resource(new_name='foo', objects=['ns1:namespace'])
k8s_resource(new_name='foo', objects=['ns2:namespace'])
`)

	f.loadErrString("k8s_resource named \"foo\" already exists")
}

func TestAdditivePortForwards(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `

k8s_yaml('foo.yaml')
k8s_resource('foo', port_forwards=8001)
k8s_resource('foo', port_forwards=8000)
`)

	f.load()
	f.assertNextManifest("foo", []model.PortForward{{LocalPort: 8001}, {LocalPort: 8000}})
}

func TestWorkloadToResourceFunction(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `

docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
def wtrf(id):
	return 'hello-' + id.name
workload_to_resource_function(wtrf)
k8s_resource('hello-foo', port_forwards=8000)
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("hello-foo", db(image("gcr.io/foo")), []model.PortForward{{LocalPort: 8000}})
}

func TestWorkloadToResourceFunctionConflict(t *testing.T) {
	f := newFixture(t)

	f.setupFooAndBar()

	f.file("Tiltfile", `

docker_build('gcr.io/foo', 'foo')
docker_build('gcr.io/bar', 'bar')
k8s_yaml(['foo.yaml', 'bar.yaml'])
def wtrf(id):
	return 'baz'
workload_to_resource_function(wtrf)
`)

	f.loadErrString("workload_to_resource_function", "bar:deployment:default:apps", "foo:deployment:default:apps", "'baz'")
}

func TestWorkloadToResourceFunctionError(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `

docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
def wtrf(id):
	return 1 + 'asdf'
workload_to_resource_function(wtrf)
k8s_resource('hello-foo', port_forwards=8000)
`)

	f.loadErrString("'foo:deployment:default:apps'", "unknown binary op: int + string", "Tiltfile:5:1", workloadToResourceFunctionN)
}

func TestWorkloadToResourceFunctionReturnsNonString(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `

docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
def wtrf(id):
	return 1
workload_to_resource_function(wtrf)
k8s_resource('hello-foo', port_forwards=8000)
`)

	f.loadErrString("'foo:deployment:default:apps'", "invalid return value", "wanted: string. got: starlark.Int", "Tiltfile:5:1", workloadToResourceFunctionN)
}

func TestWorkloadToResourceFunctionTakesNoArgs(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `

docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
def wtrf():
	return "hello"
workload_to_resource_function(wtrf)
k8s_resource('hello-foo', port_forwards=8000)
`)

	f.loadErrString("workload_to_resource_function arg must take 1 argument. wtrf takes 0")
}

func TestWorkloadToResourceFunctionTakesTwoArgs(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `

docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
def wtrf(a, b):
	return "hello"
workload_to_resource_function(wtrf)
k8s_resource('hello-foo', port_forwards=8000)
`)

	f.loadErrString("workload_to_resource_function arg must take 1 argument. wtrf takes 2")
}

func TestMultipleLiveUpdatesOnManifest(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("sancho/Dockerfile", "FROM golang:1.10")
	f.file("sidecar/Dockerfile", "FROM golang:1.10")
	f.file("sancho.yaml", testyaml.SanchoSidecarYAML) // two containers
	f.file("Tiltfile", `
k8s_yaml('sancho.yaml')
docker_build('gcr.io/some-project-162817/sancho', './sancho',
  live_update=[sync('./sancho/foo', '/bar')]
)
docker_build('gcr.io/some-project-162817/sancho-sidecar', './sidecar',
  live_update=[sync('./sidecar/baz', '/quux')]
)
`)

	sync1 := v1alpha1.LiveUpdateSync{LocalPath: filepath.Join("sancho", "foo"), ContainerPath: "/bar"}
	expectedLU1 := v1alpha1.LiveUpdateSpec{
		BasePath: f.Path(),
		Syncs:    []v1alpha1.LiveUpdateSync{sync1},
	}

	sync2 := v1alpha1.LiveUpdateSync{LocalPath: filepath.Join("sidecar", "baz"), ContainerPath: "/quux"}
	expectedLU2 := v1alpha1.LiveUpdateSpec{
		BasePath: f.Path(),
		Syncs:    []v1alpha1.LiveUpdateSync{sync2},
	}

	f.load()
	f.assertNextManifest("sancho",
		db(image("gcr.io/some-project-162817/sancho"), expectedLU1),
		db(image("gcr.io/some-project-162817/sancho-sidecar"), expectedLU2),
	)
}

func TestImpossibleLiveUpdatesOKNoLiveUpdate(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("sancho/Dockerfile", "FROM golang:1.10")
	f.file("sidecar/Dockerfile", "FROM golang:1.10")
	f.file("sancho.yaml", testyaml.SanchoSidecarYAML) // two containers
	f.file("Tiltfile", `
k8s_yaml('sancho.yaml')
docker_build('gcr.io/some-project-162817/sancho', './sancho')

# no LiveUpdate on this so nothing meriting a warning
docker_build('gcr.io/some-project-162817/sancho-sidecar', './sidecar')
`)

	// Expect no warnings!
	f.load()
}

func TestImpossibleLiveUpdatesOKSecondContainerLiveUpdate(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("sancho/Dockerfile", "FROM golang:1.10")
	f.file("sidecar/Dockerfile", "FROM golang:1.10")
	f.file("sancho.yaml", testyaml.SanchoSidecarYAML) // two containers
	f.file("Tiltfile", `
k8s_yaml('sancho.yaml')

# this is the second k8s container, but only the first image target, so should be OK
docker_build('gcr.io/some-project-162817/sancho-sidecar', './sidecar')
`)

	// Expect no warnings!
	f.load()
}

func TestTriggerModeK8S(t *testing.T) {
	for _, testCase := range []struct {
		name                string
		globalSetting       triggerMode
		k8sResourceSetting  triggerMode
		specifyAutoInit     bool
		autoInit            bool
		expectedTriggerMode model.TriggerMode
	}{
		{"default", TriggerModeUnset, TriggerModeUnset, false, false, model.TriggerModeAuto},
		{"explicit global auto", TriggerModeAuto, TriggerModeUnset, false, false, model.TriggerModeAuto},
		{"explicit global manual", TriggerModeManual, TriggerModeUnset, false, false, model.TriggerModeManualWithAutoInit},
		{"kr auto", TriggerModeUnset, TriggerModeUnset, false, false, model.TriggerModeAuto},
		{"kr manual", TriggerModeUnset, TriggerModeManual, false, false, model.TriggerModeManualWithAutoInit},
		{"kr manual, auto_init=False", TriggerModeUnset, TriggerModeManual, true, false, model.TriggerModeManual},
		{"kr manual, auto_init=True", TriggerModeUnset, TriggerModeManual, true, true, model.TriggerModeManualWithAutoInit},
		{"kr override auto", TriggerModeManual, TriggerModeAuto, false, false, model.TriggerModeAuto},
		{"kr override manual", TriggerModeAuto, TriggerModeManual, false, false, model.TriggerModeManualWithAutoInit},
		{"kr override manual, auto_init=False", TriggerModeAuto, TriggerModeManual, true, false, model.TriggerModeManual},
		{"kr override manual, auto_init=True", TriggerModeAuto, TriggerModeManual, true, true, model.TriggerModeManualWithAutoInit},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			f := newFixture(t)

			f.setupFoo()

			var globalTriggerModeDirective string
			switch testCase.globalSetting {
			case TriggerModeUnset:
				globalTriggerModeDirective = ""
			default:
				globalTriggerModeDirective = fmt.Sprintf("trigger_mode(%s)", testCase.globalSetting.String())
			}

			var k8sResourceDirective string
			switch testCase.k8sResourceSetting {
			case TriggerModeUnset:
				k8sResourceDirective = ""
			default:
				autoInitOption := ""
				if testCase.specifyAutoInit {
					autoInitOption = ", auto_init="
					if testCase.autoInit {
						autoInitOption += "True"
					} else {
						autoInitOption += "False"
					}
				}
				k8sResourceDirective = fmt.Sprintf("k8s_resource('foo', trigger_mode=%s%s)", testCase.k8sResourceSetting.String(), autoInitOption)
			}

			f.file("Tiltfile", fmt.Sprintf(`
%s
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
%s
`, globalTriggerModeDirective, k8sResourceDirective))

			f.load()

			f.assertNumManifests(1)
			f.assertNextManifest("foo", testCase.expectedTriggerMode)
		})
	}
}

func TestTriggerModeLocal(t *testing.T) {
	for _, testCase := range []struct {
		name                 string
		globalSetting        triggerMode
		localResourceSetting triggerMode
		specifyAutoInit      bool
		autoInit             bool
		expectedTriggerMode  model.TriggerMode
	}{
		{"default", TriggerModeUnset, TriggerModeUnset, false, true, model.TriggerModeAuto},
		{"explicit global auto", TriggerModeAuto, TriggerModeUnset, false, true, model.TriggerModeAuto},
		{"explicit global manual", TriggerModeManual, TriggerModeUnset, false, true, model.TriggerModeManualWithAutoInit},
		{"explicit global auto, autoInit=True", TriggerModeAuto, TriggerModeUnset, true, true, model.TriggerModeAuto},
		{"explicit global auto, autoInit=False", TriggerModeAuto, TriggerModeUnset, true, false, model.TriggerModeAutoWithManualInit},
		{"explicit global manual, autoInit=True", TriggerModeManual, TriggerModeUnset, true, true, model.TriggerModeManualWithAutoInit},
		{"explicit global manual, autoInit=False", TriggerModeManual, TriggerModeUnset, true, false, model.TriggerModeManual},
		{"local_resource auto", TriggerModeUnset, TriggerModeUnset, false, true, model.TriggerModeAuto},
		{"local_resource manual", TriggerModeUnset, TriggerModeManual, false, true, model.TriggerModeManualWithAutoInit},
		{"local_resource auto, autoInit=True", TriggerModeUnset, TriggerModeAuto, true, true, model.TriggerModeAuto},
		{"local_resource auto, autoInit=False", TriggerModeUnset, TriggerModeAuto, true, false, model.TriggerModeAutoWithManualInit},
		{"local_resource manual, autoInit=True", TriggerModeUnset, TriggerModeManual, true, true, model.TriggerModeManualWithAutoInit},
		{"local_resource manual, autoInit=False", TriggerModeUnset, TriggerModeManual, true, false, model.TriggerModeManual},
		{"local_resource override auto", TriggerModeManual, TriggerModeAuto, false, true, model.TriggerModeAuto},
		{"local_resource override manual", TriggerModeAuto, TriggerModeManual, false, true, model.TriggerModeManualWithAutoInit},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			f := newFixture(t)

			var globalTriggerModeDirective string
			switch testCase.globalSetting {
			case TriggerModeUnset:
				globalTriggerModeDirective = ""
			case TriggerModeManual:
				globalTriggerModeDirective = "trigger_mode(TRIGGER_MODE_MANUAL)"
			case TriggerModeAuto:
				globalTriggerModeDirective = "trigger_mode(TRIGGER_MODE_AUTO)"
			}

			resourceTriggerModeArg := ""
			switch testCase.localResourceSetting {
			case TriggerModeManual:
				resourceTriggerModeArg = ", trigger_mode=TRIGGER_MODE_MANUAL"
			case TriggerModeAuto:
				resourceTriggerModeArg = ", trigger_mode=TRIGGER_MODE_AUTO"
			}

			autoInitArg := ""
			if testCase.specifyAutoInit {
				if testCase.autoInit {
					autoInitArg = ", auto_init=True"
				} else {
					autoInitArg = ", auto_init=False"
				}
			}

			localResourceDirective := fmt.Sprintf("local_resource('foo', 'echo hi'%s%s)", resourceTriggerModeArg, autoInitArg)

			f.file("Tiltfile", fmt.Sprintf(`
%s
%s
`, globalTriggerModeDirective, localResourceDirective))

			f.load()

			f.assertNumManifests(1)
			f.assertNextManifest("foo", testCase.expectedTriggerMode)
		})
	}
}

func TestTriggerModeInt(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
trigger_mode(1)
`)
	f.loadErrString("got int, want TriggerMode")
}

func TestMultipleTriggerMode(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
trigger_mode(TRIGGER_MODE_MANUAL)
trigger_mode(TRIGGER_MODE_MANUAL)
`)
	f.loadErrString("trigger_mode can only be called once")
}

func TestK8sContext(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
if k8s_context() != 'fake-context':
  fail('bad context')
if k8s_namespace() != 'fake-namespace':
  fail('bad namespace')
k8s_yaml('foo.yaml')
docker_build('gcr.io/foo', 'foo')
`)

	f.load()
	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "foo.yaml")

}

func TestDockerbuildIgnoreAsString(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `

docker_build('gcr.io/foo', '.', ignore="*.txt")
k8s_yaml('foo.yaml')
`)

	f.load()
	f.assertNextManifest("foo",
		buildFilters("a.txt"),
		fileChangeFilters("a.txt"),
		buildMatches("txt.a"),
		fileChangeMatches("txt.a"),
	)
}

func TestDockerbuildIgnoreAsArray(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `

docker_build('gcr.io/foo', '.', ignore=["*.txt", "*.md"])
k8s_yaml('foo.yaml')
`)

	f.load()
	f.assertNextManifest("foo",
		buildFilters("a.txt"),
		buildFilters("a.md"),
		fileChangeFilters("a.txt"),
		fileChangeFilters("a.md"),
		buildMatches("txt.a"),
		fileChangeMatches("txt.a"),
	)
}

func TestDockerbuildInvalidIgnore(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("foo/Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("fooimage")))

	f.file("Tiltfile", `

docker_build('fooimage', 'foo', ignore=[127])
k8s_yaml('foo.yaml')
`)

	f.loadErrString("ignore must be a string or a sequence of strings; found a starlark.Int")
}

func TestDockerbuildOnly(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo', '.', only="myservice")
k8s_yaml('foo.yaml')
`)

	f.load()
	f.assertNextManifest("foo",
		buildFilters("otherservice/bar"),
		fileChangeFilters("otherservice/bar"),
		buildMatches("myservice/bar"),
		fileChangeMatches("myservice/bar"),
	)
}

func TestDockerbuildOnlyAsArray(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo', '.', only=["common", "myservice"])
k8s_yaml('foo.yaml')
`)

	f.load()
	f.assertNextManifest("foo",
		buildFilters("otherservice/bar"),
		fileChangeFilters("otherservice/bar"),
		buildMatches("myservice/bar"),
		fileChangeMatches("myservice/bar"),
		buildMatches("common/bar"),
		fileChangeMatches("common/bar"),
	)
}

func TestDockerbuildInvalidOnly(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("foo/Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("fooimage")))

	f.file("Tiltfile", `

docker_build('fooimage', 'foo', only=[127])
k8s_yaml('foo.yaml')
`)

	f.loadErrString("only must be a string or a sequence of strings; found a starlark.Int")
}

func TestDockerbuildInvalidOnlyGlob(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("foo/Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("fooimage")))

	f.file("Tiltfile", `

docker_build('fooimage', 'foo', only=["**/common"])
k8s_yaml('foo.yaml')
`)

	f.loadErrString("'only' does not support '*' file globs")
}

func TestDockerbuildOnlyAndIgnore(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo', '.', ignore="**/*.md", only=["common", "myservice"])
k8s_yaml('foo.yaml')
`)

	f.load()
	f.assertNextManifest("foo",
		buildFilters("otherservice/bar"),
		fileChangeFilters("otherservice/bar"),
		buildFilters("myservice/README.md"),
		fileChangeFilters("myservice/README.md"),
		buildMatches("myservice/bar"),
		fileChangeMatches("myservice/bar"),
		buildMatches("common/bar"),
		fileChangeMatches("common/bar"),
	)
}

// if the same file is ignored and included, the ignore takes precedence
func TestDockerbuildOnlyAndIgnoreSameFile(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo', '.', ignore="common/README.md", only="common/README.md")
k8s_yaml('foo.yaml')
`)

	f.load()
	f.assertNextManifest("foo",
		buildFilters("common/README.md"),
		fileChangeFilters("common/README.md"),
	)
}

// If an only rule starts with a !, we assume that paths starts with a !
// We don't do a double negative
func TestDockerbuildOnlyHasException(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo', '.', only="!myservice")
k8s_yaml('foo.yaml')
`)

	f.load()
	f.assertNextManifest("foo",
		buildFilters("otherservice/bar"),
		fileChangeFilters("otherservice/bar"),
		buildMatches("!myservice/bar"),
		fileChangeMatches("!myservice/bar"),
	)
}

// What if you have \n in strings?
// That's hard to make work easily, so let's just throw an error
func TestDockerbuildIgnoreWithNewline(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo', '.', ignore="\nweirdfile.txt")
k8s_yaml('foo.yaml')
`)

	f.loadErrString(`ignore cannot contain newlines; found ignore: "\nweirdfile.txt"`)
}
func TestDockerbuildOnlyWithNewline(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo', '.', only="\nweirdfile.txt")
k8s_yaml('foo.yaml')
`)

	f.loadErrString(`only cannot contain newlines; found only: "\nweirdfile.txt`)
}

// Custom Build Ignores(Single file)
func TestCustomBuildIgnoresSingular(t *testing.T) {
	f := newFixture(t)
	f.setupFoo()

	f.file("Tiltfile", `

custom_build('gcr.io/foo', 'docker build -t $EXPECTED_REF foo',
  ['foo'], ignore="a.txt")
k8s_yaml('foo.yaml')
`) // custom build doesnt support globs for dependencies
	f.load()
	f.assertNextManifest("foo",
		fileChangeFilters("foo/a.txt"),
		fileChangeMatches("foo/txt.a"),
	)
}

// Custom Build Ignores(Multiple files)
func TestCustomBuildIgnoresMultiple(t *testing.T) {
	f := newFixture(t)
	f.setupFoo()

	f.file("Tiltfile", `
custom_build('gcr.io/foo', 'docker build -t $EXPECTED_REF foo',
 ['foo'], ignore=["a.md","a.txt"])
k8s_yaml('foo.yaml')
`)
	f.load()
	f.assertNextManifest("foo",
		fileChangeFilters("foo/a.txt"),
		fileChangeFilters("foo/a.md"),
		fileChangeMatches("foo/txt.a"),
		fileChangeMatches("foo/md.a"),
	)
}

func TestEnableFeature(t *testing.T) {
	f := newFixture(t)
	f.features["testflag_disabled"] = feature.Value{Enabled: false}
	f.setupFoo()

	f.file("Tiltfile", `enable_feature('testflag_disabled')`)
	f.load()

	f.assertFeature("testflag_disabled", true)
}

func TestEnableFeatureWithError(t *testing.T) {
	f := newFixture(t)
	f.features["testflag_disabled"] = feature.Value{Enabled: false}
	f.setupFoo()

	f.file("Tiltfile", `
enable_feature('testflag_disabled')
fail('goodnight moon')
`)
	f.loadErrString("goodnight moon")

	f.assertFeature("testflag_disabled", true)
}

func TestDisableFeature(t *testing.T) {
	f := newFixture(t)
	f.features["testflag_enabled"] = feature.Value{Enabled: true}
	f.setupFoo()

	f.file("Tiltfile", `disable_feature('testflag_enabled')`)
	f.load()

	f.assertFeature("testflag_enabled", false)
}

func TestEnableFeatureThatDoesNotExist(t *testing.T) {
	f := newFixture(t)
	f.setupFoo()

	f.file("Tiltfile", `enable_feature('testflag')`)

	f.loadErrString("Unknown feature flag: testflag")
}

func TestDisableFeatureThatDoesNotExist(t *testing.T) {
	f := newFixture(t)
	f.setupFoo()

	f.file("Tiltfile", `disable_feature('testflag')`)

	f.loadErrString("Unknown feature flag: testflag")
}

func TestDisableObsoleteFeature(t *testing.T) {
	f := newFixture(t)
	f.features["obsoleteflag"] = feature.Value{Status: feature.Obsolete, Enabled: true}
	f.setupFoo()

	f.file("Tiltfile", `disable_feature('obsoleteflag')`)
	f.loadAssertWarnings("Obsolete feature flag: obsoleteflag")
}

func TestDisableSnapshots(t *testing.T) {
	f := newFixture(t)
	f.setupFoo()

	f.file("Tiltfile", `disable_snapshots()`)
	f.load()

	f.assertFeature(feature.Snapshots, false)
}

func TestDockerBuildEntrypointString(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo', '.', entrypoint="/bin/the_app")
k8s_yaml('foo.yaml')
`)

	f.load()
	f.assertNextManifest("foo", db(image("gcr.io/foo"), entrypoint(model.ToUnixCmdInDir("/bin/the_app", f.Path()))))
}

func TestDockerBuildContainerArgs(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo', '.', container_args=["bar"])
k8s_yaml('foo.yaml')
`)

	f.load()

	m := f.assertNextManifest("foo")
	assert.Equal(t,
		&v1alpha1.ImageMapOverrideArgs{Args: []string{"bar"}},
		m.ImageTargets[0].OverrideArgs)
}

func TestDockerBuildEntrypointArray(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo', '.', entrypoint=["/bin/the_app"])
k8s_yaml('foo.yaml')
`)

	f.load()
	f.assertNextManifest("foo", db(image("gcr.io/foo"), entrypoint(model.Cmd{Argv: []string{"/bin/the_app"}})))
}

func TestDockerBuild_buildArgs(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("rev.txt", "hello")
	f.file("Tiltfile", `
cmd = 'cat rev.txt'
if os.name == 'nt':
  cmd = 'type rev.txt'
docker_build('gcr.io/foo', 'foo', build_args={'GIT_REV': local(cmd)})
k8s_yaml('foo.yaml')
`)

	f.load("foo")

	m := f.assertNextManifest("foo")
	assert.Equal(t,
		[]string{"GIT_REV=hello"},
		m.ImageTargets[0].DockerBuildInfo().Args)
}

func TestCustomBuildEntrypoint(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
custom_build('gcr.io/foo', 'docker build -t $EXPECTED_REF foo',
 ['foo'], entrypoint="/bin/the_app")
k8s_yaml('foo.yaml')
`)

	f.load()
	f.assertNextManifest("foo", cb(
		image("gcr.io/foo"),
		deps(f.JoinPath("foo")),
		cmd("docker build -t $EXPECTED_REF foo", f.Path()),
		entrypoint(model.ToUnixCmdInDir("/bin/the_app", f.Path()))),
	)
}

func TestCustomBuildContainerArgs(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
custom_build('gcr.io/foo', 'docker build -t $EXPECTED_REF foo',
 ['foo'], container_args=['bar'])
k8s_yaml('foo.yaml')
`)

	f.load()
	assert.Equal(t,
		&v1alpha1.ImageMapOverrideArgs{Args: []string{"bar"}},
		f.assertNextManifest("foo").ImageTargets[0].OverrideArgs)
}

// See comments on ImageTarget#MaybeIgnoreRegistry()
func TestCustomBuildSkipsLocalDockerAndTagPassedIgnoresLocalRegistry(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
default_registry('localhost:5000')
custom_build('gcr.io/foo', ':', ["."], tag='gcr.io/foo:latest', skips_local_docker=True)
k8s_yaml('foo.yaml')
`)

	f.load()
	m := f.assertNextManifest("foo")
	refs, err := m.ImageTargets[0].Refs(f.cluster(m))
	require.NoError(t, err)
	assert.Equal(t, "gcr.io/foo", refs.ClusterRef().String())
}

func TestDuplicateYAMLEntityWithinSingleResource(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.yaml("resource.yaml",
		service("doggos"),
		service("doggos"))
	f.file("Tiltfile", `
k8s_yaml('resource.yaml')
`)
	stack := fmt.Sprintf(`Traceback (most recent call last):
  %s:2:9: in <toplevel>
  <builtin>: in k8s_yaml`, f.JoinPath("Tiltfile"))
	f.loadErrString(tiltfile_k8s.DuplicateYAMLDetectedError("Service doggos", stack).Error())
}

func TestDuplicateYAMLEntityWithinSingleResourceAllowed(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.yaml("resource.yaml",
		service("doggos"),
		service("doggos"))
	f.file("Tiltfile", `
k8s_yaml('resource.yaml', allow_duplicates=True)
`)
	f.load()
}

func TestDuplicateYAMLEntityAcrossResources(t *testing.T) {
	f := newFixture(t)

	f.dockerfile("foo1/Dockerfile")
	f.yaml("foo1.yaml", deployment("foo", image("gcr.io/foo1"), namespace("ns1")))
	f.file("Tiltfile", `

k8s_yaml(['foo1.yaml', 'foo1.yaml'])
k8s_resource('foo:deployment:ns1', new_name='foo')
`)

	stack := fmt.Sprintf(`Traceback (most recent call last):
  %s:3:9: in <toplevel>
  <builtin>: in k8s_yaml`, f.JoinPath("Tiltfile"))
	f.loadErrString(tiltfile_k8s.DuplicateYAMLDetectedError("Deployment foo (Namespace: ns1)", stack).Error())
}

func TestDuplicateYAMLEntityInSingleWorkload(t *testing.T) {
	//Services corresponding to a deployment get pulled into the same resource.
	f := newFixture(t)

	labelsFoo := map[string]string{"foo": "bar"}
	f.yaml("all.yaml",
		deployment("foo-deployment", image("gcr.io/foo1"), namespace("ns1"), withLabels(labelsFoo)),
		service("foo-service", withLabels(labelsFoo)),
		service("foo-service", withLabels(labelsFoo)))
	f.file("Tiltfile", `
k8s_yaml('all.yaml')
`)

	stack := fmt.Sprintf(`Traceback (most recent call last):
  %s:2:9: in <toplevel>
  <builtin>: in k8s_yaml`, f.JoinPath("Tiltfile"))
	f.loadErrString(tiltfile_k8s.DuplicateYAMLDetectedError("Service foo-service", stack).Error())
}

func TestDuplicateYAMLEntityInUserAssembledNonWorkloadResource(t *testing.T) {
	f := newFixture(t)
	f.gitInit("")
	f.yaml("all.yaml",
		service("foo-service"),
		service("foo-service"))
	f.file("Tiltfile", `
k8s_yaml('all.yaml')
k8s_resource(objects=['foo-service:Service:default'], new_name='my-services')
`)

	stack := fmt.Sprintf(`Traceback (most recent call last):
  %s:2:9: in <toplevel>
  <builtin>: in k8s_yaml`, f.JoinPath("Tiltfile"))
	f.loadErrString(tiltfile_k8s.DuplicateYAMLDetectedError("Service foo-service", stack).Error())
}

func TestSetTeamID(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", "set_team('sharks')")
	f.load()

	assert.Equal(t, "sharks", f.loadResult.TeamID)
}

func TestSetTeamIDEmpty(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", "set_team('')")
	f.loadErrString("team_id cannot be empty")
}

func TestSetTeamIDMultiple(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
set_team('sharks')
set_team('jets')
`)
	f.loadErrString("team_id set multiple times", "'sharks'", "'jets'")
}

func TestK8SContextAcceptance(t *testing.T) {
	for _, test := range []struct {
		name                    string
		contextName             k8s.KubeContext
		env                     clusterid.Product
		expectError             bool
		expectedErrorSubstrings []string
	}{
		{"minikube", "minikube", clusterid.ProductMinikube, false, nil},
		{"docker-for-desktop", "docker-for-desktop", clusterid.ProductDockerDesktop, false, nil},
		{"kind", "KIND", clusterid.ProductKIND, false, nil},
		{"gke", "gke", clusterid.ProductGKE, true, []string{"'gke'", "If you're sure", "switch k8s contexts", "allow_k8s_contexts"}},
		{"allowed", "allowed-context", clusterid.ProductGKE, false, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)

			f.file("Tiltfile", `
k8s_yaml("foo.yaml")
allow_k8s_contexts("allowed-context")
`)
			f.setupFoo()

			f.k8sContext = test.contextName
			f.k8sEnv = test.env
			if !test.expectError {
				f.load()
			} else {
				f.loadErrString(test.expectedErrorSubstrings...)
			}
		})
	}
}

// Test for fix to https://github.com/tilt-dev/tilt/issues/4234
func TestCheckK8SContextWhenOnlyUncategorizedK8s(t *testing.T) {
	f := newFixture(t)

	// We'll only have Uncategorized k8s entities, no K8s resources--
	// make sure we still check K8sContext and throw an error if need be
	f.yaml("service.yaml", service("some-service"))

	f.file("Tiltfile", `
k8s_yaml("service.yaml")
allow_k8s_contexts("allowed-context")
`)
	f.setupFoo()

	f.k8sContext = "gke"
	f.k8sEnv = clusterid.ProductGKE

	f.loadErrString("If you're sure", "switch k8s contexts", "allow_k8s_contexts")
}

func TestLocalObeysAllowedK8sContexts(t *testing.T) {
	for _, test := range []struct {
		name                    string
		contextName             k8s.KubeContext
		env                     clusterid.Product
		expectError             bool
		expectedErrorSubstrings []string
	}{
		{"gke", "gke", clusterid.ProductGKE, true, []string{"'gke'", "If you're sure", "switch k8s contexts", "allow_k8s_contexts"}},
		{"allowed", "allowed-context", clusterid.ProductGKE, false, nil},
		{"docker-compose", "unknown", k8s.ProductNone, false, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)

			f.file("Tiltfile", `
allow_k8s_contexts("allowed-context")
local('echo hi')
`)
			f.setupFoo()

			f.k8sContext = test.contextName
			f.k8sEnv = test.env
			if !test.expectError {
				f.load()
			} else {
				f.loadErrString(test.expectedErrorSubstrings...)
			}
		})
	}
}

func TestLocalResourceOnlyUpdateCmd(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("test", "echo hi", deps=["foo/bar", "foo/a.txt"])
`)

	f.setupFoo()
	f.file(".gitignore", "*.txt")
	f.load()

	f.assertNumManifests(1)
	path1 := "foo/bar"
	path2 := "foo/a.txt"
	m := f.assertNextManifest("test", localTarget(updateCmd(f.Path(), "echo hi", nil), deps(path1, path2)), fileChangeMatches("foo/a.txt"))

	lt := m.LocalTarget()
	assert.Equal(t, []v1alpha1.IgnoreDef{
		{BasePath: f.JoinPath(".git")},
	}, lt.GetFileWatchIgnores())

	f.assertConfigFiles("Tiltfile", ".tiltignore")
}

func TestLocalResourceOnlyServeCmd(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("test", serve_cmd="sleep 1000")
`)

	f.load()

	f.assertNumManifests(1)
	f.assertNextManifest("test", localTarget(serveCmd(f.Path(), "sleep 1000", nil)))

	f.assertConfigFiles("Tiltfile", ".tiltignore")
}

func TestLocalResourceUpdateAndServeCmd(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("test", cmd="echo hi", serve_cmd="sleep 1000")
`)

	f.load()

	f.assertNumManifests(1)
	f.assertNextManifest("test", localTarget(
		updateCmd(f.Path(), "echo hi", nil),
		serveCmd(f.Path(), "sleep 1000", nil),
	))

	f.assertConfigFiles("Tiltfile", ".tiltignore")
}

func TestLocalResourceNeitherUpdateOrServeCmd(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("test")
`)

	f.loadErrString("local_resource must have a cmd and/or a serve_cmd, but both were empty")
}

func TestLocalResourceUpdateCmdArray(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("test", ["echo", "hi"])
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("test", localTarget(updateCmdArray(f.Path(), []string{"echo", "hi"}, nil)))
}

func TestLocalResourceServeCmdArray(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("test", serve_cmd=["echo", "hi"])
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("test", localTarget(serveCmdArray(f.Path(), []string{"echo", "hi"}, nil)))
}

func TestLocalResourceWorkdir(t *testing.T) {
	f := newFixture(t)

	f.file("nested/Tiltfile", `
local_resource("nested-local", "echo nested", deps=["foo/bar", "more_nested/repo"])
`)
	f.file("Tiltfile", `
include('nested/Tiltfile')
local_resource("toplvl-local", "echo hello world", deps=["foo/baz", "foo/a.txt"])
`)

	f.setupFoo()
	f.MkdirAll("nested/.git")
	f.MkdirAll("nested/more_nested/repo/.git")
	f.MkdirAll("foo/baz/.git")
	f.MkdirAll("foo/.git") // no Tiltfile lives here, nor is it a LocalResource dep; won't be pulled in as a repo
	f.load()

	f.assertNumManifests(2)
	mNested := f.assertNextManifest("nested-local",
		localTarget(updateCmd(f.JoinPath("nested"), "echo nested", nil),
			deps("nested/foo/bar", "nested/more_nested/repo")))

	ltNested := mNested.LocalTarget()
	assert.Equal(t, []v1alpha1.IgnoreDef{
		{BasePath: f.JoinPath("nested/more_nested/repo", ".git")},
		{BasePath: f.JoinPath("nested", ".git")},
	}, ltNested.GetFileWatchIgnores())

	mTop := f.assertNextManifest("toplvl-local", localTarget(updateCmd(f.Path(), "echo hello world", nil), deps("foo/baz", "foo/a.txt")))
	ltTop := mTop.LocalTarget()
	assert.Equal(t, []v1alpha1.IgnoreDef{
		{BasePath: f.JoinPath("foo/baz", ".git")},
		{BasePath: f.JoinPath(".git")},
	}, ltTop.GetFileWatchIgnores())
}

func TestLocalResourceIgnore(t *testing.T) {
	f := newFixture(t)

	f.file(".dockerignore", "**/**.c")
	f.file("Tiltfile", "include('proj/Tiltfile')")
	f.file("proj/Tiltfile", `
local_resource("test", "echo hi", deps=["foo"], ignore=["**/*.a", "foo/bar.d"])
`)

	f.setupFoo()
	f.file(".gitignore", "*.txt")
	f.load()

	m := f.assertNextManifest("test")

	// TODO(dmiller): I can't figure out how to translate these in to (file\build)(Matches\Filters) assert functions
	filter := ignore.CreateFileChangeFilter(m.LocalTarget().GetFileWatchIgnores())

	for _, tc := range []struct {
		path        string
		expectMatch bool
	}{
		{"proj/foo/bar.a", true},
		{"proj/foo/bar.b", false},
		{"proj/foo/baz/bar.a", true},
		{"proj/foo/bar.d", true},
	} {
		matches, err := filter.Matches(f.JoinPath(tc.path))
		require.NoError(t, err)
		require.Equal(t, tc.expectMatch, matches, tc.path)
	}
}

func TestLocalResourceUpdateCmdEnv(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("test", "echo hi", env={"KEY1": "value1", "KEY2": "value2"}, serve_cmd="sleep 1000")
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("test", localTarget(
		updateCmd(f.Path(), "echo hi", []string{"KEY1=value1", "KEY2=value2"}),
		serveCmd(f.Path(), "sleep 1000", nil),
	))
}

func TestLocalResourceServeCmdEnv(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("test", "echo hi", serve_cmd="sleep 1000", serve_env={"KEY1": "value1", "KEY2": "value2"})
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("test", localTarget(
		updateCmd(f.Path(), "echo hi", nil),
		serveCmd(f.Path(), "sleep 1000", []string{"KEY1=value1", "KEY2=value2"}),
	))
}

func TestLocalResourceUpdateCmdDir(t *testing.T) {
	f := newFixture(t)

	f.file("nested/inside.txt", "inside the nested directory")
	f.file("Tiltfile", `
local_resource("test", cmd="cat inside.txt", dir="nested")
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("test", localTarget(
		updateCmd(f.JoinPath(f.Path(), "nested"), "cat inside.txt", nil),
	))
}

func TestLocalResourceUpdateCmdDirNone(t *testing.T) {
	f := newFixture(t)

	f.file("here.txt", "same level")
	f.file("Tiltfile", `
local_resource("test", cmd="cat here.txt", dir=None)
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("test", localTarget(
		updateCmd(f.Path(), "cat here.txt", nil),
	))
}

func TestLocalResourceServeCmdDir(t *testing.T) {
	f := newFixture(t)

	f.file("nested/inside.txt", "inside the nested directory")
	f.file("Tiltfile", `
local_resource("test", serve_cmd="cat inside.txt", serve_dir="nested")
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("test", localTarget(
		serveCmd(f.JoinPath(f.Path(), "nested"), "cat inside.txt", nil),
	))
}

func TestLocalResourceServeCmdDirNone(t *testing.T) {
	f := newFixture(t)

	f.file("here.txt", "same level")
	f.file("Tiltfile", `
local_resource("test", serve_cmd="cat here.txt", serve_dir=None)
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("test", localTarget(
		serveCmd(f.Path(), "cat here.txt", nil),
	))
}

func TestCustomBuildStoresTiltfilePath(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `include('proj/Tiltfile')
k8s_yaml("foo.yaml")`)
	f.file("proj/Tiltfile", `
custom_build(
  'gcr.io/foo',
  'build.sh',
  ['foo']
)
`)
	f.file("proj/build.sh", "docker build -t $EXPECTED_REF gcr.io/foo")
	f.file("proj/Dockerfile", "FROM alpine")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("foo", cb(
		image("gcr.io/foo"),
		deps(f.JoinPath("proj/foo")),
		cmd("build.sh", f.JoinPath("proj")),
	))
}

func TestSecretString(t *testing.T) {
	f := newFixture(t)

	f.file("secret.yaml", `
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
stringData:
  client-id: hello
  client-secret: world
`)
	f.file("Tiltfile", `
k8s_yaml('secret.yaml')
`)

	f.load()

	secrets := f.loadResult.Secrets
	assert.Equal(t, 2, len(secrets))
	assert.Equal(t, "client-id", secrets["hello"].Key)
	assert.Equal(t, "hello", string(secrets["hello"].Value))
	assert.Equal(t, "aGVsbG8=", string(secrets["hello"].ValueEncoded))
	assert.Equal(t, "client-secret", secrets["world"].Key)
	assert.Equal(t, "world", string(secrets["world"].Value))
	assert.Equal(t, "d29ybGQ=", string(secrets["world"].ValueEncoded))
}

func TestSecretBytes(t *testing.T) {
	f := newFixture(t)

	f.file("secret.yaml", `
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
data:
  client-id: aGVsbG8=
  client-secret: d29ybGQ=
`)
	f.file("Tiltfile", `
k8s_yaml('secret.yaml')
`)

	f.load()

	secrets := f.loadResult.Secrets
	assert.Equal(t, 2, len(secrets))
	assert.Equal(t, "client-id", secrets["hello"].Key)
	assert.Equal(t, "hello", string(secrets["hello"].Value))
	assert.Equal(t, "aGVsbG8=", string(secrets["hello"].ValueEncoded))
	assert.Equal(t, "client-secret", secrets["world"].Key)
	assert.Equal(t, "world", string(secrets["world"].Value))
	assert.Equal(t, "d29ybGQ=", string(secrets["world"].ValueEncoded))
}

func TestSecretSettingsDisableScrub(t *testing.T) {
	f := newFixture(t)

	f.file("secret.yaml", `
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
stringData:
  client-id: hello
  client-secret: world
`)
	f.file("Tiltfile", `
k8s_yaml('secret.yaml')
secret_settings(disable_scrub=True)
`)

	f.load()

	secrets := f.loadResult.Secrets
	assert.Empty(t, secrets, "expect no secrets to be collected if scrubbing secrets is disabled")
}

func TestDockerPruneSettings(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
docker_prune_settings(max_age_mins=111, num_builds=222)
`)

	f.load()
	res := f.loadResult.DockerPruneSettings

	assert.True(t, res.Enabled)
	assert.Equal(t, time.Minute*111, res.MaxAge)
	assert.Equal(t, 222, res.NumBuilds)
	assert.Equal(t, model.DockerPruneDefaultInterval, res.Interval) // default
}

func TestDockerPruneSettingsDefaultsWhenCalled(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
docker_prune_settings(num_builds=123)
`)

	f.load()
	res := f.loadResult.DockerPruneSettings

	assert.True(t, res.Enabled)
	assert.Equal(t, model.DockerPruneDefaultMaxAge, res.MaxAge)
	assert.Equal(t, 123, res.NumBuilds)
	assert.Equal(t, model.DockerPruneDefaultInterval, res.Interval)
}

func TestDockerPruneSettingsDefaultsWhenNotCalled(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
print('nothing to see here')
`)

	f.load()
	res := f.loadResult.DockerPruneSettings

	assert.True(t, res.Enabled)
	assert.Equal(t, model.DockerPruneDefaultMaxAge, res.MaxAge)
	assert.Equal(t, 0, res.NumBuilds)
	assert.Equal(t, model.DockerPruneDefaultInterval, res.Interval)
}

func TestK8SDependsOn(t *testing.T) {
	f := newFixture(t)

	f.setupFooAndBar()
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')

docker_build('gcr.io/bar', 'bar')
k8s_yaml('bar.yaml')
k8s_resource('bar', resource_deps=['foo'])
`)

	f.load()
	f.assertNextManifest("foo", resourceDeps())
	f.assertNextManifest("bar", resourceDeps("foo"))
}

func TestLocalDependsOn(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource('foo', 'echo foo')
local_resource('bar', 'echo bar', resource_deps=['foo'])
`)

	f.load()
	f.assertNextManifest("foo", resourceDeps())
	f.assertNextManifest("bar", resourceDeps("foo"))
}

func TestDependsOnMissingResource(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource('baz', 'echo baz')
local_resource('bar', 'echo bar', resource_deps=['foo', 'baz'])
`)

	f.loadAssertWarnings("resource bar specified a dependency on unknown resource foo - dependency ignored")
	f.assertNumManifests(2)
	f.assertNextManifest("baz", resourceDeps())
	f.assertNextManifest("bar", resourceDeps("baz"))
}

func TestDependsOnSelf(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource('bar', 'echo bar', resource_deps=['bar'])
`)

	f.loadErrString("resource bar specified a dependency on itself")
}

func TestDependsOnCycle(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource('foo', 'echo foo', resource_deps=['baz'])
local_resource('bar', 'echo bar', resource_deps=['foo'])
local_resource('baz', 'echo baz', resource_deps=['bar'])
`)

	f.loadErrString("cycle detected in resource dependency graph", "bar -> foo", "foo -> baz", "baz -> bar")
}

func TestDependsOnPulledInOnPartialLoad(t *testing.T) {
	for _, tc := range []struct {
		name            string
		resourcesToLoad []model.ManifestName
		expected        []model.ManifestName
	}{
		{
			name:            "a",
			resourcesToLoad: []model.ManifestName{"a"},
			expected:        []model.ManifestName{"a"},
		},
		{
			name:            "c",
			resourcesToLoad: []model.ManifestName{"c"},
			expected:        []model.ManifestName{"a", "b", "c"},
		},
		{
			name:            "d, e",
			resourcesToLoad: []model.ManifestName{"d", "e"},
			expected:        []model.ManifestName{"a", "b", "d", "e"},
		},
		{
			name:            "e",
			resourcesToLoad: []model.ManifestName{"e"},
			expected:        []model.ManifestName{"e"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)

			f.file("Tiltfile", `
local_resource('a', 'echo a')
local_resource('b', 'echo b', resource_deps=['a'])
local_resource('c', 'echo c', resource_deps=['b'])
local_resource('d', 'echo d', resource_deps=['b'])
local_resource('e', 'echo e')
`)

			var args []string
			for _, r := range tc.resourcesToLoad {
				args = append(args, string(r))
			}
			f.load(args...)
			require.Equal(t, tc.expected, f.loadResult.EnabledManifests)
		})
	}
}

func TestLocalResourceAllowParallel(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("a", ["echo", "hi"], allow_parallel=True)
local_resource("b", ["echo", "hi"])
local_resource("c", serve_cmd=["echo", "hi"])
`)

	f.load()
	a := f.assertNextManifest("a")
	assert.True(t, a.LocalTarget().AllowParallel)
	b := f.assertNextManifest("b")
	assert.False(t, b.LocalTarget().AllowParallel)

	// local_resource serve_cmd is currently modeled as a no-op local cmd that
	// triggers a server restart. It's always OK for those no-op local cmds to
	// run in parallel.
	c := f.assertNextManifest("c")
	assert.True(t, c.LocalTarget().AllowParallel)
}

func TestLocalResourceInvalidName(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("a/b", ["echo", "hi"])
`)

	f.loadErrString(
		// Verify the validation message
		"invalid value \"a/b\": may not contain '/'",

		// Verify the stack trace points to the local_resource
		"Tiltfile:2:15: in <toplevel>")
}

func TestMaxParallelUpdates(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		tiltfile                   string
		expectErrorContains        string
		expectedMaxParallelUpdates int
	}{
		{
			name:                       "default value if func not called",
			tiltfile:                   "print('hello world')",
			expectedMaxParallelUpdates: model.DefaultMaxParallelUpdates,
		},
		{
			name:                       "default value if arg not specified",
			tiltfile:                   "update_settings(k8s_upsert_timeout_secs=123)",
			expectedMaxParallelUpdates: model.DefaultMaxParallelUpdates,
		},
		{
			name:                       "set max parallel updates",
			tiltfile:                   "update_settings(max_parallel_updates=42)",
			expectedMaxParallelUpdates: 42,
		},
		{
			name:                "NaN error",
			tiltfile:            "update_settings(max_parallel_updates='boop')",
			expectErrorContains: "got starlark.String, want int",
		},
		{
			name:                "must be positive int",
			tiltfile:            "update_settings(max_parallel_updates=-1)",
			expectErrorContains: "must be >= 1",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)

			f.file("Tiltfile", tc.tiltfile)

			if tc.expectErrorContains != "" {
				f.loadErrString(tc.expectErrorContains)
				return
			}

			f.load()
			actualBuildSlots := f.loadResult.UpdateSettings.MaxParallelUpdates()
			assert.Equal(t, tc.expectedMaxParallelUpdates, actualBuildSlots, "expected vs. actual maxParallelUpdates")
		})
	}
}

func TestK8sUpsertTimeout(t *testing.T) {
	for _, tc := range []struct {
		name                string
		tiltfile            string
		expectErrorContains string
		expectedTimeout     time.Duration
	}{
		{
			name:            "default value if func not called",
			tiltfile:        "print('hello world')",
			expectedTimeout: v1alpha1.KubernetesApplyTimeoutDefault,
		},
		{
			name:            "default value if arg not specified",
			tiltfile:        "update_settings(max_parallel_updates=123)",
			expectedTimeout: v1alpha1.KubernetesApplyTimeoutDefault,
		},
		{
			name:            "set max parallel updates",
			tiltfile:        "update_settings(k8s_upsert_timeout_secs=42)",
			expectedTimeout: 42 * time.Second,
		},
		{
			name:                "NaN error",
			tiltfile:            "update_settings(k8s_upsert_timeout_secs='boop')",
			expectErrorContains: "got starlark.String, want int",
		},
		{
			name:                "must be positive int",
			tiltfile:            "update_settings(k8s_upsert_timeout_secs=-1)",
			expectErrorContains: "minimum k8s upsert timeout is 1s",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)

			f.file("Tiltfile", tc.tiltfile)

			if tc.expectErrorContains != "" {
				f.loadErrString(tc.expectErrorContains)
				return
			}

			f.load()
			actualTimeout := f.loadResult.UpdateSettings.K8sUpsertTimeout()
			assert.Equal(t, tc.expectedTimeout, actualTimeout, "expected vs. actual k8sUpsertTimeout")
		})
	}
}

func TestUpdateSettingsCalledTwice(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `update_settings(max_parallel_updates=123)
update_settings(k8s_upsert_timeout_secs=456)`)

	f.load()
	assert.Equal(t, 123, f.loadResult.UpdateSettings.MaxParallelUpdates(), "expected vs. actual MaxParallelUpdates")
	assert.Equal(t, 456*time.Second, f.loadResult.UpdateSettings.K8sUpsertTimeout(), "expected vs. actual k8sUpsertTimeout")
}

// recursion is disabled by default in Starlark. Make sure we've enabled it for Tiltfiles.
func TestRecursionEnabled(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
def fact(n):
  if not n:
    return 1
  return n * fact(n - 1)

print("fact: %d" % (fact(10)))
`)

	f.load()

	require.Contains(t, f.out.String(), fmt.Sprintf("fact: %d", 10*9*8*7*6*5*4*3*2*1))
}

func TestBuiltinAnalytics(t *testing.T) {
	f := newFixture(t)

	// covering:
	// 1. a positional arg
	// 2. a keyword arg
	// 3. a mix of both for the same arg
	// 4. a builtin from a starkit plugin
	// 5. loading a Tilt plugin

	f.file("Tiltfile", `
local('echo hi')
local(command='echo hi')
local('echo hi', quiet=True)
allow_k8s_contexts("hello")
load('ext://fooExt', 'printFoo')
`)

	// write the plugin locally so we don't need to deal with fake fetchers etc.
	f.WriteFile(f.JoinPath("tilt-extensions", "fooExt", "Tiltfile"), `
def printFoo():
  print("foo")
`)

	f.load()

	countEvent := f.SingleAnalyticsEvent("tiltfile.loaded")

	// make sure it has all the expected builtin call counts
	expectedCounts := map[string]string{
		"tiltfile.invoked.local":                           "3",
		"tiltfile.invoked.local.arg.command":               "3",
		"tiltfile.invoked.local.arg.quiet":                 "1",
		"tiltfile.invoked.allow_k8s_contexts":              "1",
		"tiltfile.invoked.allow_k8s_contexts.arg.contexts": "1",
	}

	for k, v := range expectedCounts {
		require.Equal(t, v, countEvent.Tags[k], "count for %s", k)
	}

	pluginEvent := f.SingleAnalyticsEvent("tiltfile.loaded.plugin")
	expectedTags := map[string]string{
		"ext_name": "fooExt",
		"env":      "docker-for-desktop",
	}
	require.Equal(t, expectedTags, pluginEvent.Tags)
}

func TestCustomTagsReported(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
experimental_analytics_report({'foo': 'bar'})
`)

	f.load()

	countEvent := f.SingleAnalyticsEvent("tiltfile.custom.report")

	require.Equal(t, map[string]string{"foo": "bar"}, countEvent.Tags)
}

func TestK8sResourceObjectsAddsNonWorkload(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("secret.yaml", secret("bar"))
	f.yaml("namespace.yaml", namespace("baz"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_yaml('secret.yaml')
k8s_yaml('namespace.yaml')
k8s_resource('foo', objects=['bar', 'baz:namespace:default'])
`)

	f.load()

	f.assertNextManifest("foo", deployment("foo"), k8sObject("bar", "Secret"), k8sObject("baz", "Namespace"),
		podReadiness(model.PodReadinessWait))
	f.assertNoMoreManifests()
}

func TestK8sResourceObjectsWithSameName(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("secret.yaml", secret("bar"))
	f.yaml("namespace.yaml", namespace("bar"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_yaml('secret.yaml')
k8s_yaml('namespace.yaml')
k8s_resource('foo', objects=['bar', 'bar:namespace:default'])
`)

	f.loadErrString("\"bar\" is not a unique fragment. Objects that match \"bar\" are \"bar:Secret:default\", \"bar:Namespace:default\"")
}

func TestK8sResourceObjectsCantIncludeSameObjectTwice(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("secret1.yaml", secret("bar"))
	f.yaml("secret2.yaml", secret("qux"))
	f.yaml("namespace.yaml", namespace("bar"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_yaml('secret1.yaml')
k8s_yaml('secret2.yaml')
k8s_resource('foo', objects=['bar', 'bar:secret:default'])
`)

	f.loadErrString("No object identified by the fragment \"bar:secret:default\" could be found in remaining YAML. Valid remaining fragments are: \"qux:Secret:default\"")
}

func TestK8sResourceObjectsMultipleAmbiguous(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("secret.yaml", secret("bar"))
	f.yaml("namespace.yaml", namespace("bar"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_yaml('secret.yaml')
k8s_yaml('namespace.yaml')
k8s_resource('foo', objects=['bar', 'bar'])
`)

	f.loadErrString("bar\" is not a unique fragment. Objects that match \"bar\" are \"bar:Secret:default\", \"bar:Namespace:default\"")
}

func TestK8sResourceObjectEmptySelector(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("secret.yaml", secret("bar"))
	f.yaml("namespace.yaml", namespace("baz"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_yaml('secret.yaml')
k8s_yaml('namespace.yaml')
k8s_resource('foo', objects=[''])
`)

	f.loadErrString("Error making selector from string \"\"")
}

func TestK8sResourceObjectInvalidSelector(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("secret.yaml", secret("bar"))
	f.yaml("namespace.yaml", namespace("baz"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_yaml('secret.yaml')
k8s_yaml('namespace.yaml')
k8s_resource('foo', objects=['baz:namespace:default:wot'])
`)

	f.loadErrString("Error making selector from string \"baz:namespace:default:wot\"")
}

func TestK8sResourceObjectSelectorWithEscapedColon(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("secret.yaml", secret("quu:bar"))
	f.yaml("namespace.yaml", namespace("baz"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_yaml('secret.yaml')
k8s_yaml('namespace.yaml')
k8s_resource('foo', objects=['quu\\:bar', 'baz:namespace:default'])
`)

	f.load()

	f.assertNextManifest("foo", deployment("foo"), k8sObject("quu:bar", "Secret"), k8sObject("baz", "Namespace"),
		podReadiness(model.PodReadinessWait))
	f.assertNoMoreManifests()
}

func TestK8sResourceObjectSelectorWithEscapedBackslash(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("secret.yaml", secret("quu\\bar"))
	f.yaml("namespace.yaml", namespace("baz"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_yaml('secret.yaml')
k8s_yaml('namespace.yaml')
k8s_resource('foo', objects=['quu\\\\bar', 'baz:namespace:default'])
`)

	f.load()

	f.assertNextManifest("foo", deployment("foo"), k8sObject("quu\\bar", "Secret"), k8sObject("baz", "Namespace"),
		podReadiness(model.PodReadinessWait))
	f.assertNoMoreManifests()
}

func TestK8sResourceObjectSelectorSuggestedObjectsAreEscaped(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("secret.yaml", secret("quu:bar"))
	f.yaml("namespace.yaml", namespace("baz"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_yaml('secret.yaml')
k8s_yaml('namespace.yaml')
k8s_resource('foo', objects=['quu:bar', 'baz:namespace:default'])
`)

	f.loadErrString(
		`No object identified by the fragment "quu:bar" could be found`,
		`Possible objects are: "foo:Deployment:default", "quu\\:bar:Secret:default", "baz:Namespace:default"`,
	)
}

func TestK8sResourceObjectSelectorInvalidEscapeSequence(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("secret.yaml", secret("quu:bar"))
	f.yaml("namespace.yaml", namespace("baz"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_yaml('secret.yaml')
k8s_yaml('namespace.yaml')
k8s_resource('foo', objects=['qu\\u:bar', 'baz:namespace:default'])
`)

	f.loadErrString(
		`invalid escape sequence '\u' in 'qu\u:b'`,
	)
}

func TestK8sResourceObjectIncludesSelectorThatDoesntExist(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("secret.yaml", secret("bar"))
	f.yaml("namespace.yaml", namespace("baz"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_yaml('secret.yaml')
k8s_yaml('namespace.yaml')
k8s_resource('foo', objects=['baz:secret:default'])
`)

	f.loadErrString("No object identified by the fragment \"baz:secret:default\" could be found. Possible objects are: \"foo:Deployment:default\", \"bar:Secret:default\", \"baz:Namespace:default\"")
}

func TestK8sResourceObjectsPartialNames(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("secret.yaml", secret("bar"))
	f.yaml("namespace.yaml", namespace("bar"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_yaml('secret.yaml')
k8s_yaml('namespace.yaml')
k8s_resource('foo', objects=['bar:secret', 'bar:namespace'])
`)

	f.load()
	f.assertNextManifest("foo", deployment("foo"), k8sObject("bar", "Secret"), k8sObject("bar", "Namespace"))
	f.assertNoMoreManifests()
}

func TestK8sResourcePrefixesShouldntMatch(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("secret.yaml", secret("bar"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_yaml('secret.yaml')
k8s_resource('foo', objects=['ba'])
`)

	f.loadErrString("No object identified by the fragment \"ba\" could be found. Possible objects are: \"foo:Deployment:default\", \"bar:Secret:default\"")
}

func TestK8sResourceAmbiguousSelector(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("secret.yaml", secret("bar"))
	f.yaml("namespace.yaml", namespace("bar"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_yaml('secret.yaml')
k8s_yaml('namespace.yaml')
k8s_resource('foo', objects=['bar'])
`)

	f.loadErrString("\"bar\" is not a unique fragment. Objects that match \"bar\" are \"bar:Secret:default\", \"bar:Namespace:default\"")
}

func TestK8sResourceObjectDuplicate(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("secret.yaml", secret("bar"))
	f.yaml("anotherworkload.yaml", deployment("baz"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_yaml('anotherworkload.yaml')
k8s_yaml('secret.yaml')
k8s_resource('foo', objects=['bar'])
k8s_resource('baz', objects=['bar'])
`)

	f.loadErrString("No object identified by the fragment \"bar\" could be found in remaining YAML. Valid remaining fragments are:")
}

func TestK8sResourceObjectMultipleResources(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("secret.yaml", secret("bar"))
	f.yaml("namespace.yaml", namespace("qux"))
	f.yaml("anotherworkload.yaml", deployment("baz"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_yaml('secret.yaml')
k8s_yaml('namespace.yaml')
k8s_yaml('anotherworkload.yaml')
k8s_resource('foo', objects=['bar'])
k8s_resource('baz')
`)

	f.load()
	f.assertNextManifest("foo", deployment("foo"), k8sObject("bar", "Secret"))
	f.assertNextManifest("baz", deployment("baz"))
	f.assertNextManifestUnresourced("qux")
	f.assertNoMoreManifests()
}

func TestMultipleResourcesMultipleObjects(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("secret.yaml", secret("bar"))
	f.yaml("namespace.yaml", namespace("qux"))
	f.yaml("anotherworkload.yaml", deployment("baz"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_yaml('secret.yaml')
k8s_yaml('namespace.yaml')
k8s_yaml('anotherworkload.yaml')
k8s_resource('foo', objects=['bar'])
k8s_resource('baz', objects=['qux'])
`)

	f.load()
	f.assertNextManifest("foo", deployment("foo"), k8sObject("bar", "Secret"))
	f.assertNextManifest("baz", deployment("baz"), namespace("qux"))
	f.assertNoMoreManifests()
}

func TestK8sResourceAmbiguousWorkloadAmbiguousObject(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("secret.yaml", secret("foo"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_yaml('secret.yaml')
k8s_resource('foo', objects=['foo'])
`)

	f.loadErrString("\"foo\" is not a unique fragment. Objects that match \"foo\" are \"foo:Deployment:default\", \"foo:Secret:default\"")
}

func TestK8sResourceObjectsWithWorkloadToResourceFunction(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("secret.yaml", secret("foo"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_yaml('secret.yaml')
def wtrf(id):
	return 'hello-' + id.name
workload_to_resource_function(wtrf)
k8s_resource('hello-foo', objects=['foo:secret'])
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("hello-foo", k8sObject("foo", "Secret"))
	f.assertNoMoreManifests()
}

func TestK8sResourceNewNameWithoutObjects(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
k8s_resource(new_name='foo')
`)

	f.loadErrString("k8s_resource doesn't specify a workload or any objects")
}

func TestK8sResourceObjectsWithGroup(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("secret.yaml", secret("bar"))
	f.yaml("namespace.yaml", namespace("baz"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_yaml('secret.yaml')
k8s_yaml('namespace.yaml')
k8s_resource('foo', objects=['bar', 'baz:namespace:default:core'])
`)

	// TODO(dmiller): see comment on fullNameFromK8sEntity for info on why we don't support specifying group right now
	f.loadErrString("Error making selector from string \"baz:namespace:default:core\": Too many parts in selector. Selectors must contain between 1 and 3 parts (colon separated), found 4 parts in baz:namespace:default:core")
	// f.assertNextManifest("foo", deployment("foo"), k8sObject("bar", "Secret"), k8sObject("baz", "Namespace"))
	// f.assertNoMoreManifests()
}

func TestK8sResourceObjectClusterScoped(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("namespace.yaml", namespace("baz"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_yaml('namespace.yaml')
k8s_resource('foo', objects=['baz:namespace'])
`)

	f.load()

	f.assertNextManifest("foo", deployment("foo"), k8sObject("baz", "Namespace"))
	f.assertNoMoreManifests()
}

// TODO(dmiller): I'm not sure if this makes sense ... cluster scoped things like namespaces _can't_ have
// namespaces, so should we allow you to specify namespaces for them?
// For now we just leave them as "default"
func TestK8sResourceObjectClusterScopedWithNamespace(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("namespace.yaml", namespace("baz"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_yaml('namespace.yaml')
k8s_resource('foo', objects=['baz:namespace:qux'])
`)

	f.loadErrString("No object identified by the fragment \"baz:namespace:qux\" could be found. Possible objects are: \"foo:Deployment:default\", \"baz:Namespace:default\"")
}

func TestK8sResourceObjectsNonWorkloadOnly(t *testing.T) {
	f := newFixture(t)

	f.yaml("secret.yaml", secret("bar"))
	f.yaml("namespace.yaml", namespace("baz"))

	f.file("Tiltfile", `
k8s_yaml('secret.yaml')
k8s_yaml('namespace.yaml')
k8s_resource(new_name='foo', objects=['bar', 'baz:namespace:default'])
`)

	f.load()

	f.assertNextManifest("foo", k8sObject("bar", "Secret"), k8sObject("baz", "Namespace"), podReadiness(model.PodReadinessIgnore))
	f.assertNoMoreManifests()
}

func TestK8sResourceNewNameAdditive(t *testing.T) {
	f := newFixture(t)

	f.yaml("a.yaml", namespace("a"))
	f.yaml("b.yaml", namespace("b"))

	f.file("Tiltfile", `
k8s_yaml('a.yaml')
k8s_yaml('b.yaml')
k8s_resource(new_name='namespaces', objects=['a'])
k8s_resource('namespaces', objects=['b'])
`)

	f.load()
	f.assertNextManifest("namespaces", k8sObject("a", "Namespace"), k8sObject("b", "Namespace"))
}

func TestK8sExistingResourceAdditive(t *testing.T) {
	f := newFixture(t)

	f.yaml("a.yaml", deployment("a"))
	f.yaml("b.yaml", namespace("b"))
	f.yaml("c.yaml", namespace("c"))

	f.file("Tiltfile", `
k8s_yaml('a.yaml')
k8s_yaml('b.yaml')
k8s_yaml('c.yaml')
k8s_resource('a', objects=['b'])
k8s_resource('a', objects=['c'])
`)

	f.load()
	f.assertNextManifest("a",
		k8sObject("a", "Deployment"), k8sObject("b", "Namespace"), k8sObject("c", "Namespace"))
}

func TestK8sExistingResourceNewNameAdditive(t *testing.T) {
	f := newFixture(t)

	// this was working non-deterministically based on hashtable order, so generate a bunch of resources
	// to reduce the chance of false positives
	// https://github.com/tilt-dev/tilt/issues/4808
	for i := 1; i <= 25; i++ {
		f.yaml(fmt.Sprintf("deploy%d.yaml", i), deployment(fmt.Sprintf("deploy%d", i)))
	}

	f.file("Tiltfile", `
for i in range(1, 26):
  k8s_yaml('deploy%d.yaml' % (i))
  k8s_resource('deploy%d' % (i), new_name='deploy%d-renamed' % (i), labels=['a'])
  k8s_resource('deploy%d-renamed' % (i), labels=['b'])
`)

	f.load()
	for i := 1; i <= 25; i++ {
		f.assertNextManifest(model.ManifestName(fmt.Sprintf("deploy%d-renamed", i)),
			k8sObject(fmt.Sprintf("deploy%d", i), "Deployment"), resourceLabels("a", "b"))
	}
}

func TestK8sExistingResourceNewNameAlreadyTaken(t *testing.T) {
	f := newFixture(t)

	f.yaml("a.yaml", deployment("a"))
	f.yaml("b.yaml", namespace("b"))
	f.yaml("c.yaml", namespace("c"))

	f.file("Tiltfile", `
k8s_yaml('a.yaml')
k8s_yaml('b.yaml')
k8s_yaml('c.yaml')
k8s_resource('a', objects=['b'])
k8s_resource(new_name='a', objects=['c'])
`)

	f.loadErrString(`k8s_resource named "a" already exists`)
}

func TestK8sNonWorkloadOnlyResourceWithAllTheOptions(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("secret.yaml", secret("bar"))
	f.yaml("namespace.yaml", namespace("baz"))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_yaml('secret.yaml')
k8s_yaml('namespace.yaml')
k8s_resource(new_name='bar', objects=['bar', 'baz:namespace:default'], port_forwards=9876, extra_pod_selectors=[{'quux': 'corge'}], trigger_mode=TRIGGER_MODE_MANUAL, resource_deps=['foo'])
`)

	f.load()

	f.assertNextManifest("foo")
	f.assertNextManifest("bar", k8sObject("bar", "Secret"), k8sObject("baz", "Namespace"))
	f.assertNoMoreManifests()
}

func TestK8sResourceEmptyWorkloadSpecifierAndNoObjects(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `

k8s_yaml('foo.yaml')
k8s_resource('', port_forwards=8000)
`)

	f.loadErrString("Resource name missing. Must give a name for an existing resource or a new_name to create a new resource.")
}

func TestK8sResourceNonWorkloadRequiresNewName(t *testing.T) {
	f := newFixture(t)

	f.yaml("secret.yaml", secret("bar"))
	f.yaml("namespace.yaml", namespace("baz"))

	f.file("Tiltfile", `
k8s_yaml('secret.yaml')
k8s_yaml('namespace.yaml')
k8s_resource(objects=['bar', 'baz:namespace:default'])
`)

	f.loadErrString("Resource name missing. Must give a name for an existing resource or a new_name to create a new resource.")
}

func TestK8sResourceNewNameCantOverwriteWorkload(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()
	f.yaml("secret.yaml", secret("bar"))

	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
k8s_yaml('secret.yaml')
k8s_resource('foo', new_name='bar')
k8s_resource(new_name='bar', objects=['bar:secret'])
`)

	// NOTE(dmiller): because `range`ing over maps is unstable we don't know which error we will encounter:
	// 1. Trying to create a non-workload resource when a resource by that name already exists
	// 2. Trying to rename a resource to a name that already exists
	// so we match a string that appears in both error messages
	f.loadErrString("already exists")
}

func TestK8sResourceObjectsNonAmbiguousDefaultNamespace(t *testing.T) {
	f := newFixture(t)

	f.file("serving-core.yaml", testyaml.KnativeServingCore)

	f.file("Tiltfile", `
k8s_yaml([
	'serving-core.yaml',
])

k8s_resource(
  objects=[
	  'queue-proxy:Image',
	],
  new_name='knative-gateways')
`)

	f.load()
	f.assertNextManifest("knative-gateways")
	f.assertNoMoreManifests()
}

func TestK8sResourceObjectsAreNotCaseSensitive(t *testing.T) {
	f := newFixture(t)

	f.file("serving-core.yaml", testyaml.KnativeServingCore)

	f.file("Tiltfile", `
k8s_yaml([
	'serving-core.yaml',
])

k8s_resource(
  objects=[
	  'queue-proxy:image',
	],
  new_name='knative-gateways')
`)

	f.load()
	f.assertNextManifest("knative-gateways")
	f.assertNoMoreManifests()
}

func TestK8sResourceLabels(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
k8s_resource('foo', labels="test")
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("foo", resourceLabels("test"))
}

func TestK8sResourceLabelsAppend(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
k8s_resource('foo', labels="test")
k8s_resource('foo', labels="test2")
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("foo", resourceLabels("test", "test2"))
}

func TestLocalResourceLabels(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("test", cmd="echo hi", labels="foo")
local_resource("test2", cmd="echo hi2", labels=["bar", "baz"])
`)

	f.load()
	f.assertNumManifests(2)
	f.assertNextManifest("test", resourceLabels("foo"))
	f.assertNextManifest("test2", resourceLabels("bar", "baz"))
}

// https://github.com/tilt-dev/tilt/issues/5467
func TestLoadErrorWithArgs(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", "asdf")
	f.loadArgsErrString([]string{"foo"}, "undefined: asdf")
}

func TestContentsChangedTag(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", "print('Hello')")
	tiltfile := ctrltiltfile.MainTiltfile(f.JoinPath("Tiltfile"), []string{})
	loader := f.newTiltfileLoader()

	// *.changed = false on first load (no previous hash values)
	tlr := loader.Load(f.ctx, tiltfile, nil)
	assert.Equal(t, "0d4b93146f79968657afdad8b23d423973bf7a7e97690d146e6b6cfcc24e617e", tlr.Hashes.TiltfileSHA256)
	assert.Equal(t, "0d4b93146f79968657afdad8b23d423973bf7a7e97690d146e6b6cfcc24e617e", tlr.Hashes.AllFilesSHA256)

	event := f.SingleAnalyticsEvent("tiltfile.loaded")
	assert.Equal(t, "false", event.Tags["tiltfile.changed"])
	assert.Equal(t, "false", event.Tags["allfiles.changed"])

	// *.changed = true because hash values differ
	f.an.Counts = []analytics.CountEvent{}
	tlr.Hashes = hasher.Hashes{TiltfileSHA256: "abc123", AllFilesSHA256: "abc123"}
	tlr = loader.Load(f.ctx, tiltfile, &tlr)
	event = f.SingleAnalyticsEvent("tiltfile.loaded")
	assert.Equal(t, "true", event.Tags["tiltfile.changed"])
	assert.Equal(t, "true", event.Tags["allfiles.changed"])

	// *.changed = false because hash values match
	f.an.Counts = []analytics.CountEvent{}
	tlr = loader.Load(f.ctx, tiltfile, &tlr)
	event = f.SingleAnalyticsEvent("tiltfile.loaded")
	assert.Equal(t, "false", event.Tags["tiltfile.changed"])
	assert.Equal(t, "false", event.Tags["allfiles.changed"])
}

type fixture struct {
	ctx context.Context
	out *bytes.Buffer
	t   *testing.T
	*tempdir.TempDirFixture
	k8sContext   k8s.KubeContext
	k8sNamespace k8s.Namespace
	k8sEnv       clusterid.Product
	webHost      model.WebHost

	ta *tiltanalytics.TiltAnalytics
	an *analytics.MemoryAnalytics

	loadResult TiltfileLoadResult
	warnings   []string
	features   feature.Defaults
}

func (f *fixture) newTiltfileLoader() TiltfileLoader {
	dcc := dockercompose.NewDockerComposeClient(docker.LocalEnv{})

	k8sContextPlugin := k8scontext.NewPlugin(f.k8sContext, f.k8sNamespace, f.k8sEnv)
	versionPlugin := version.NewPlugin(model.TiltBuild{Version: "0.5.0"})
	configPlugin := config.NewPlugin("up")
	localEnv := localexec.DefaultEnv(12345, f.webHost)
	execer := localexec.NewProcessExecer(localEnv)
	extr := tiltextension.NewFakeExtReconciler(f.Path())
	extrr := tiltextension.NewFakeExtRepoReconciler(f.Path())
	extPlugin := tiltextension.NewFakePlugin(extrr, extr)
	ciSettingsPlugin := cisettings.NewPlugin(0)
	return ProvideTiltfileLoader(f.ta, k8sContextPlugin, versionPlugin, configPlugin,
		extPlugin, ciSettingsPlugin, dcc, f.webHost, execer, f.features, f.k8sEnv)
}

func newFixture(t *testing.T) *fixture {
	out := new(bytes.Buffer)
	ctx, ma, ta := testutils.ForkedCtxAndAnalyticsForTest(out)
	f := tempdir.NewTempDirFixture(t)
	f.Chdir()

	// copy the features to avoid unintentional mutation by tests
	features := make(feature.Defaults)
	for k, v := range feature.MainDefaults {
		features[k] = v
	}

	r := &fixture{
		ctx:            ctx,
		out:            out,
		t:              t,
		TempDirFixture: f,
		an:             ma,
		ta:             ta,
		k8sContext:     "fake-context",
		k8sNamespace:   "fake-namespace",
		k8sEnv:         clusterid.ProductDockerDesktop,
		features:       features,
	}

	// Collect the warnings
	l := logger.NewFuncLogger(false, logger.DebugLvl, func(level logger.Level, fields logger.Fields, msg []byte) error {
		if level == logger.WarnLvl {
			r.warnings = append(r.warnings, string(msg))
		}
		out.Write(msg)
		return nil
	})
	r.ctx = logger.WithLogger(r.ctx, l)

	return r
}

func (f *fixture) file(path string, contents string) {
	f.WriteFile(path, contents)
}

type k8sOpts interface{}

func (f *fixture) dockerfile(path string) {
	f.file(path, simpleDockerfile)
}

func (f *fixture) dockerignore(path string) {
	f.file(path, simpleDockerignore)
}

func (f *fixture) yaml(path string, entities ...k8sOpts) {
	var entityObjs []k8s.K8sEntity

	for _, e := range entities {
		switch e := e.(type) {
		case deploymentHelper:
			s := testyaml.SnackYaml
			if e.image != "" {
				s = strings.ReplaceAll(s, testyaml.SnackImage, e.image)
			}
			s = strings.ReplaceAll(s, testyaml.SnackName, e.name)
			objs, err := k8s.ParseYAMLFromString(s)
			if err != nil {
				f.t.Fatal(err)
			}

			if len(e.templateLabels) > 0 {
				for i, obj := range objs {
					withLabels, err := k8s.OverwriteLabels(obj, model.ToLabelPairs(e.templateLabels))
					if err != nil {
						f.t.Fatal(err)
					}
					objs[i] = withLabels
				}
			}

			for i, obj := range objs {
				de := obj.Obj.(*appsv1.Deployment)
				for i, c := range de.Spec.Template.Spec.Containers {
					for _, ev := range e.envVars {
						c.Env = append(c.Env, v1.EnvVar{
							Name:  ev.name,
							Value: ev.value,
						})
					}
					de.Spec.Template.Spec.Containers[i] = c
				}
				if e.namespace != "" {
					de.Namespace = e.namespace
				}
				obj.Obj = de
				objs[i] = obj
			}

			entityObjs = append(entityObjs, objs...)
		case serviceHelper:
			s := testyaml.DoggosServiceYaml
			s = strings.ReplaceAll(s, testyaml.DoggosName, e.name)
			objs, err := k8s.ParseYAMLFromString(s)
			if err != nil {
				f.t.Fatal(err)
			}

			if e.selectorLabels != nil {
				for _, obj := range objs {
					err := overwriteSelectorsForService(&obj, e.selectorLabels)
					if err != nil {
						f.t.Fatal(err)
					}
				}
			}

			entityObjs = append(entityObjs, objs...)

		case secretHelper:
			s := testyaml.SecretYaml
			s = strings.ReplaceAll(s, testyaml.SecretName, e.name)
			objs, err := k8s.ParseYAMLFromString(s)
			if err != nil {
				f.t.Fatal(err)
			}

			entityObjs = append(entityObjs, objs...)
		case namespaceHelper:
			s := testyaml.MyNamespaceYAML
			s = strings.ReplaceAll(s, testyaml.MyNamespaceName, e.namespace)
			objs, err := k8s.ParseYAMLFromString(s)
			if err != nil {
				f.t.Fatal(err)
			}
			entityObjs = append(entityObjs, objs...)
		default:
			f.t.Fatalf("unexpected entity %T %v", e, e)
		}
	}

	s, err := k8s.SerializeSpecYAML(entityObjs)
	if err != nil {
		f.t.Fatal(err)
	}
	f.file(path, s)
}

// Default load. Fails if there are any warnings.
func (f *fixture) load(args ...string) {
	f.t.Helper()
	f.loadAllowWarnings(args...)
	if len(f.warnings) != 0 {
		f.t.Fatalf("Unexpected warnings. Actual: %s", f.warnings)
	}
}

// Load the manifests, expecting warnings.
// Warnings should be asserted later with assertWarnings
func (f *fixture) loadAllowWarnings(args ...string) {
	f.t.Helper()
	tlr := f.newTiltfileLoader().Load(f.ctx, ctrltiltfile.MainTiltfile(f.JoinPath("Tiltfile"), args), nil)
	err := tlr.Error
	if err != nil {
		f.t.Fatal(err)
	}
	f.loadResult = tlr
	require.NoError(f.t, model.InferImageProperties(f.loadResult.Manifests))
}

func unusedImageWarning(unusedImage string, suggestedImages []string, configType string) string {
	ret := fmt.Sprintf("Image not used in any %s config:\n    ✕ %s", configType, unusedImage)
	if len(suggestedImages) > 0 {
		ret += "\nDid you mean…"
		for _, s := range suggestedImages {
			ret += fmt.Sprintf("\n    - %s", s)
		}
	}
	ret += "\nSkipping this image build"
	ret += fmt.Sprintf("\nIf this is deliberate, suppress this warning with: update_settings(suppress_unused_image_warnings=[%q])", unusedImage)
	return ret
}

// Load the manifests, expecting warnings.
func (f *fixture) loadAssertWarnings(warnings ...string) {
	f.loadAllowWarnings()
	f.assertWarnings(warnings...)
}

func (f *fixture) loadErrString(msgs ...string) {
	f.loadArgsErrString(nil, msgs...)
}

func (f *fixture) loadArgsErrString(args []string, msgs ...string) {
	f.t.Helper()
	tlr := f.newTiltfileLoader().Load(f.ctx, ctrltiltfile.MainTiltfile(f.JoinPath("Tiltfile"), args), nil)
	err := tlr.Error

	if err == nil {
		f.t.Fatalf("expected error but got nil")
	}
	f.loadResult = tlr
	errText := err.Error()

	for _, msg := range msgs {
		if !strings.Contains(errText, msg) {
			f.t.Fatalf("error %q does not contain string %q", errText, msg)
		}
	}
	require.NoError(f.t, model.InferImageProperties(tlr.Manifests))
}

func (f *fixture) gitInit(path string) {
	if err := os.MkdirAll(f.JoinPath(path, ".git"), os.FileMode(0777)); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) assertNoMoreManifests() {
	if len(f.loadResult.Manifests) != 0 {
		names := make([]string, len(f.loadResult.Manifests))
		for i, m := range f.loadResult.Manifests {
			names[i] = m.Name.String()
		}
		f.t.Fatalf("expected no more manifests but found %d: %s",
			len(names), strings.Join(names, ", "))
	}
}

// Helper func for asserting that the next manifest is Unresourced
// k8s YAML containing the given k8s entities.
func (f *fixture) assertNextManifestUnresourced(expectedEntities ...string) model.Manifest {
	lowercaseExpected := []string{}
	for _, e := range expectedEntities {
		lowercaseExpected = append(lowercaseExpected, strings.ToLower(e))
	}
	next := f.assertNextManifest(model.UnresourcedYAMLManifestName)

	entities, err := k8s.ParseYAML(bytes.NewBufferString(next.K8sTarget().YAML))
	assert.NoError(f.t, err)

	entityNames := make([]string, len(entities))
	for i, e := range entities {
		entityNames[i] = strings.ToLower(e.Name())
	}
	assert.Equal(f.t, lowercaseExpected, entityNames)
	return next
}

type funcOpt func(*testing.T, model.Manifest) bool

// assert functions and helpers
func (f *fixture) assertNextManifest(name model.ManifestName, opts ...interface{}) model.Manifest {
	f.t.Helper()

	if len(f.loadResult.Manifests) == 0 {
		f.t.Fatalf("no more manifests; trying to find %q (did you call `f.load`?)", name)
	}

	m := f.loadResult.Manifests[0]
	if m.Name != name {
		f.t.Fatalf("expected next manifest to be '%s' but found '%s'", name, m.Name)
	}

	f.loadResult.Manifests = f.loadResult.Manifests[1:]

	imageIndex := 0
	nextImageTarget := func() model.ImageTarget {
		ret := m.ImageTargetAt(imageIndex)
		imageIndex++
		return ret
	}

	for _, opt := range opts {
		switch opt := opt.(type) {
		case dbHelper:
			image := nextImageTarget()

			refs, err := image.Refs(f.cluster(m))
			require.NoError(f.t, err, "Determining image refs")
			ref := refs.ConfigurationRef
			if ref.Empty() {
				f.t.Fatalf("manifest %v has no more image refs; expected %q", m.Name, opt.image.ref)
			}

			expectedConfigRef := container.MustParseNamed(opt.image.ref)
			if !assert.Equal(f.t, expectedConfigRef.String(), ref.String(), "manifest %v image ref", m.Name) {
				f.t.FailNow()
			}

			expectedLocalRef := container.MustParseNamed(opt.image.localRef)
			require.Equal(f.t, expectedLocalRef.String(), refs.LocalRef().String(), "manifest %v localRef", m.Name)

			if opt.image.clusterRef != "" {
				expectedClusterRef := container.MustParseNamed(opt.image.clusterRef)
				require.Equal(f.t, expectedClusterRef.String(), refs.ClusterRef().String(), "manifest %v clusterRef", m.Name)
			}

			assert.Equal(f.t, opt.image.matchInEnvVars, image.MatchInEnvVars)

			if !image.IsDockerBuild() {
				f.t.Fatalf("expected docker build but manifest %v has no docker build info", m.Name)
			}

			for _, matcher := range opt.matchers {
				switch matcher := matcher.(type) {
				case entrypointHelper:
					if !sliceutils.StringSliceEquals(matcher.cmd.Argv, image.OverrideCommand.Command) {
						f.t.Fatalf("expected OverrideCommand (aka entrypoint) %v, got %v",
							matcher.cmd.Argv, image.OverrideCommand.Command)
					}
				case v1alpha1.LiveUpdateSpec:
					lu := image.LiveUpdateSpec
					assert.False(f.t, liveupdate.IsEmptySpec(lu))
					assert.Equal(f.t, matcher, lu)
				default:
					f.t.Fatalf("unknown dbHelper matcher: %T %v", matcher, matcher)
				}
			}
		case cbHelper:
			image := nextImageTarget()

			refs, err := image.Refs(f.cluster(m))
			require.NoError(f.t, err, "Determining image refs")

			ref := refs.ConfigurationRef
			expectedRef := container.MustParseNamed(opt.image.ref)
			if !assert.Equal(f.t, expectedRef.String(), ref.String(), "manifest %v image ref", m.Name) {
				f.t.FailNow()
			}

			if !image.IsCustomBuild() {
				f.t.Fatalf("Expected custom build but manifest %v has no custom build info", m.Name)
			}
			cbInfo := image.CustomBuildInfo()

			for _, matcher := range opt.matchers {
				switch matcher := matcher.(type) {
				case depsHelper:
					assert.Equal(f.t, matcher.deps, cbInfo.Deps)
				case cmdHelper:
					assert.Equal(f.t, matcher.cmd.Argv, cbInfo.Args)
				case tagHelper:
					assert.Equal(f.t, matcher.tag, cbInfo.OutputTag)
				case disablePushHelper:
					assert.Equal(f.t, matcher.disabled, cbInfo.OutputMode == v1alpha1.CmdImageOutputLocalDockerAndRemote)
				case entrypointHelper:
					if !sliceutils.StringSliceEquals(matcher.cmd.Argv, image.OverrideCommand.Command) {
						f.t.Fatalf("expected OverrideCommand (aka entrypoint) %v, got %v",
							matcher.cmd.Argv, image.OverrideCommand.Command)
					}
				case v1alpha1.LiveUpdateSpec:
					lu := image.LiveUpdateSpec
					assert.False(f.t, liveupdate.IsEmptySpec(lu))
					assert.Equal(f.t, matcher, lu)
				}
			}

		case deploymentHelper:
			yaml := m.K8sTarget().YAML
			found := false
			for _, e := range f.entities(yaml) {
				if e.GVK().Kind == "Deployment" && e.Name() == opt.name {
					found = true
					break
				}
			}
			if !found {
				f.t.Fatalf("deployment %v not found in yaml %q", opt.name, yaml)
			}
		case v1alpha1.KubernetesDiscoveryStrategy:
			assert.Equal(f.t, opt, m.K8sTarget().DiscoveryStrategy)
		case podReadinessHelper:
			assert.Equal(f.t, opt.podReadiness, m.K8sTarget().PodReadinessMode)
		case namespaceHelper:
			yaml := m.K8sTarget().YAML
			found := false
			for _, e := range f.entities(yaml) {
				if e.GVK().Kind == "Namespace" && e.Name() == opt.namespace {
					found = true
					break
				}
			}
			if !found {
				f.t.Fatalf("namespace %s not found in yaml %q", opt.namespace, yaml)
			}
		case serviceHelper:
			yaml := m.K8sTarget().YAML
			found := false
			for _, e := range f.entities(yaml) {
				if e.GVK().Kind == "Service" && e.Name() == opt.name {
					found = true
					break
				}
			}
			if !found {
				f.t.Fatalf("service %v not found in yaml %q", opt.name, yaml)
			}
		case k8sObjectHelper:
			yaml := m.K8sTarget().YAML
			found := false
			for _, e := range f.entities(yaml) {
				if e.GVK().Kind == opt.kind && e.Name() == opt.name {
					found = true
					break
				}
			}
			if !found {
				f.t.Fatalf("entity of kind %s with name %s not found in yaml %q", opt.kind, opt.name, yaml)
			}
		case extraPodSelectorsHelper:
			actual := m.K8sTarget().KubernetesApplySpec.KubernetesDiscoveryTemplateSpec.ExtraSelectors
			assert.ElementsMatch(f.t, k8s.SetsAsLabelSelectors(opt.labels), actual)
		case numEntitiesHelper:
			yaml := m.K8sTarget().YAML
			entities := f.entities(yaml)
			if opt.num != len(f.entities(yaml)) {
				f.t.Fatalf("manifest %v has %v entities in %v; expected %v", m.Name, len(entities), yaml, opt.num)
			}

		case matchPathHelper:
			// Make sure the paths matches one of the syncs.
			isDep := false
			path := f.JoinPath(opt.path)
			for _, d := range m.LocalPaths() {
				if ospath.IsChild(d, path) {
					isDep = true
				}
			}

			if !isDep {
				f.t.Errorf("Path %s is not a dependency of manifest %s", path, m.Name)
			}

			expectedFilter := opt.missing

			var filterName string
			var filter model.PathMatcher
			if opt.fileChange {
				filter = ignore.CreateFileChangeFilter(m.ImageTargetAt(0).GetFileWatchIgnores())
				filterName = "FileChangeFilter"
			} else {
				db, ok := m.ImageTargetAt(0).BuildDetails.(model.DockerBuild)
				if !ok {
					f.t.Fatalf("BuildContextFilter only applies to docker_build")
				}
				filter = ignore.CreateBuildContextFilter(db.DockerImageSpec.ContextIgnores)
				filterName = "BuildContextFilter"
			}

			actualFilter, err := filter.Matches(path)
			if err != nil {
				f.t.Fatalf("Error matching filter (%s): %v", path, err)
			}
			if actualFilter != expectedFilter {
				if expectedFilter {
					f.t.Errorf("%s should filter %s", filterName, path)
				} else {
					f.t.Errorf("%s should not filter %s", filterName, path)
				}
			}

		case []model.PortForward:
			if len(opt) == 0 {
				assert.Nil(f.t, m.K8sTarget().KubernetesApplySpec.PortForwardTemplateSpec)
			} else {
				var expectedForwards []v1alpha1.Forward
				for _, pf := range opt {
					expectedForwards = append(expectedForwards, v1alpha1.Forward{
						LocalPort:     int32(pf.LocalPort),
						ContainerPort: int32(pf.ContainerPort),
						Host:          pf.Host,
						Name:          pf.Name,
						Path:          pf.PathForAppend(),
					})
				}
				assert.ElementsMatch(f.t,
					expectedForwards,
					m.K8sTarget().KubernetesApplySpec.PortForwardTemplateSpec.Forwards)
			}
		case dcResourceLinks:
			f.assertLinks(opt, m.DockerComposeTarget().Links)
		case localResourceLinks:
			f.assertLinks(opt, m.LocalTarget().Links)
		case k8sResourceLinks:
			f.assertLinks(opt, m.K8sTarget().Links)
		case model.TriggerMode:
			assert.Equal(f.t, opt, m.TriggerMode)
		case resourceDependenciesHelper:
			assert.Equal(f.t, opt.deps, m.ResourceDependencies)
		case funcOpt:
			assert.True(f.t, opt(f.t, m))
		case localTargetHelper:
			lt := m.LocalTarget()
			for _, matcher := range opt.matchers {
				switch matcher := matcher.(type) {
				case updateCmdHelper:
					assert.Equal(f.t, matcher.cmd.Argv, lt.UpdateCmdSpec.Args)
					assert.Equal(f.t, matcher.cmd.Dir, lt.UpdateCmdSpec.Dir)
					assert.Equal(f.t, matcher.cmd.Env, lt.UpdateCmdSpec.Env)
				case serveCmdHelper:
					assert.Equal(f.t, matcher.cmd, lt.ServeCmd)
				case depsHelper:
					deps := f.JoinPaths(matcher.deps)
					assert.ElementsMatch(f.t, deps, lt.Dependencies())
				case readinessProbeHelper:
					assert.EqualValues(f.t, matcher.probeSpec, lt.ReadinessProbe)
				default:
					f.t.Fatalf("unknown matcher for local target %T", matcher)
				}
			}
		case resourceLabelsHelper:
			assert.Equal(f.t, opt.labels, m.Labels)
		default:
			f.t.Fatalf("unexpected arg to assertNextManifest: %T %v", opt, opt)
		}
	}

	f.assertManifestConsistency(m)

	return m
}

// All manifests currently contain redundant information
// such that each Deploy target lists its image ID dependencies.
func (f *fixture) assertManifestConsistency(m model.Manifest) {
	iTargetIDs := map[model.TargetID]bool{}
	for _, iTarget := range m.ImageTargets {
		if iTargetIDs[iTarget.ID()] {
			f.t.Fatalf("Image Target %s appears twice in manifest: %s", iTarget.ID(), m.Name)
		}
		iTargetIDs[iTarget.ID()] = true
	}

	deployTarget := m.DeployTarget
	for _, depID := range deployTarget.DependencyIDs() {
		if !iTargetIDs[depID] {
			f.t.Fatalf("Image Target needed by deploy target is missing: %s", depID)
		}
	}
}

func (f *fixture) imageTargetNames(m model.Manifest) []string {
	result := []string{}
	for _, iTarget := range m.ImageTargets {
		result = append(result, iTarget.ID().Name.String())
	}
	return result
}

func (f *fixture) idNames(ids []model.TargetID) []string {
	result := []string{}
	for _, id := range ids {
		result = append(result, id.Name.String())
	}
	return result
}

func (f *fixture) assertNumManifests(expected int) {
	assert.Equal(f.t, expected, len(f.loadResult.Manifests))
}

func (f *fixture) assertConfigFiles(filenames ...string) {
	f.t.Helper()
	var expected []string
	for _, filename := range filenames {
		expected = append(expected, f.JoinPath(filename))
	}
	sort.Strings(expected)
	sort.Strings(f.loadResult.ConfigFiles)
	assert.Equal(f.t, expected, f.loadResult.ConfigFiles)
}

func (f *fixture) assertWarnings(warnings ...string) {
	var expected []string
	for _, warning := range warnings {
		expected = append(expected, warning+"\n")
	}
	sort.Strings(expected)
	sort.Strings(f.warnings)
	assert.Equal(f.t, expected, f.warnings)
}

func (f *fixture) entities(y string) []k8s.K8sEntity {
	es, err := k8s.ParseYAMLFromString(y)
	if err != nil {
		f.t.Fatal(err)
	}
	return es
}

func (f *fixture) assertFeature(key string, enabled bool) {
	assert.Equal(f.t, enabled, f.loadResult.FeatureFlags[key])
}

func (f *fixture) assertLinks(expected, actual []model.Link) {
	require.Len(f.t, actual, len(expected), "comparing # of links")
	for i, exp := range expected {
		require.Equalf(f.t, exp.URLString(), actual[i].URLString(), "link at index %d", i)
		require.Equalf(f.t, exp.Name, actual[i].Name, "link at index %d", i)
	}
}

func (f *fixture) cluster(m model.Manifest) *v1alpha1.Cluster {
	f.t.Helper()

	tlr := f.loadResult

	if m.IsK8s() {
		return &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: v1alpha1.ClusterNameDefault,
			},
			Spec: v1alpha1.ClusterSpec{
				Connection: &v1alpha1.ClusterConnection{
					Kubernetes: &v1alpha1.KubernetesClusterConnection{},
				},
				DefaultRegistry: tlr.DefaultRegistry,
			},
		}
	}

	if m.IsDC() {
		return &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: v1alpha1.ClusterNameDocker,
			},
			Spec: v1alpha1.ClusterSpec{
				Connection: &v1alpha1.ClusterConnection{
					Docker: &v1alpha1.DockerClusterConnection{},
				},
				DefaultRegistry: tlr.DefaultRegistry,
			},
		}
	}

	return &v1alpha1.Cluster{}
}

type secretHelper struct {
	name string
}

func secret(name string) secretHelper {
	return secretHelper{name: name}
}

type namespaceHelper struct {
	namespace string
}

func namespace(namespace string) namespaceHelper {
	return namespaceHelper{namespace}
}

type deploymentHelper struct {
	name           string
	image          string
	templateLabels map[string]string
	envVars        []envVar
	namespace      string
}

func deployment(name string, opts ...interface{}) deploymentHelper {
	r := deploymentHelper{name: name}
	for _, opt := range opts {
		switch opt := opt.(type) {
		case imageHelper:
			r.image = opt.ref
		case labelsHelper:
			r.templateLabels = opt.labels
		case envVarHelper:
			r.envVars = opt.envVars
		case namespaceHelper:
			r.namespace = opt.namespace
		default:
			panic(fmt.Errorf("unexpected arg to deployment: %T %v", opt, opt))
		}
	}
	return r
}

type podReadinessHelper struct {
	podReadiness model.PodReadinessMode
}

func podReadiness(podReadiness model.PodReadinessMode) podReadinessHelper {
	return podReadinessHelper{podReadiness: podReadiness}
}

type serviceHelper struct {
	name           string
	selectorLabels map[string]string
}

func service(name string, opts ...interface{}) serviceHelper {
	r := serviceHelper{name: name}
	for _, opt := range opts {
		switch opt := opt.(type) {
		case labelsHelper:
			r.selectorLabels = opt.labels
		default:
			panic(fmt.Errorf("unexpected arg to deployment: %T %v", opt, opt))
		}
	}
	return r
}

type k8sObjectHelper struct {
	name string
	kind string
}

func k8sObject(name string, kind string) k8sObjectHelper {
	return k8sObjectHelper{name: name, kind: kind}
}

type extraPodSelectorsHelper struct {
	labels []labels.Set
}

func extraPodSelectors(labelSets ...labels.Set) extraPodSelectorsHelper {
	ret := extraPodSelectorsHelper{
		labels: append([]labels.Set(nil), labelSets...),
	}
	return ret
}

type numEntitiesHelper struct {
	num int
}

func numEntities(num int) numEntitiesHelper {
	return numEntitiesHelper{num}
}

type matchPathHelper struct {
	path       string
	missing    bool
	fileChange bool
}

func buildMatches(path string) matchPathHelper {
	return matchPathHelper{
		path: path,
	}
}

func buildFilters(path string) matchPathHelper {
	return matchPathHelper{
		path:    path,
		missing: true,
	}
}

func fileChangeMatches(path string) matchPathHelper {
	return matchPathHelper{
		path:       path,
		fileChange: true,
	}
}

func fileChangeFilters(path string) matchPathHelper {
	return matchPathHelper{
		path:       path,
		missing:    true,
		fileChange: true,
	}
}

type resourceDependenciesHelper struct {
	deps []model.ManifestName
}

func resourceDeps(deps ...string) resourceDependenciesHelper {
	var mns []model.ManifestName
	for _, d := range deps {
		mns = append(mns, model.ManifestName(d))
	}
	return resourceDependenciesHelper{deps: mns}
}

type resourceLabelsHelper struct {
	labels map[string]string
}

func resourceLabels(labels ...string) resourceLabelsHelper {
	ret := resourceLabelsHelper{
		labels: map[string]string{},
	}
	for _, l := range labels {
		ret.labels[l] = l
	}
	return ret
}

type imageHelper struct {
	ref            string
	localRef       string
	clusterRef     string
	matchInEnvVars bool
}

func image(ref string) imageHelper {
	return imageHelper{ref: ref, localRef: ref}
}

func (ih imageHelper) withLocalRef(localRef string) imageHelper {
	ih.localRef = localRef
	return ih
}

func (ih imageHelper) withClusterRef(clusterRef string) imageHelper {
	ih.clusterRef = clusterRef
	return ih
}

func (ih imageHelper) withMatchInEnvVars() imageHelper {
	ih.matchInEnvVars = true
	return ih
}

type labelsHelper struct {
	labels map[string]string
}

func withLabels(labels map[string]string) labelsHelper {
	return labelsHelper{labels: labels}
}

type envVar struct {
	name  string
	value string
}

type envVarHelper struct {
	envVars []envVar
}

// usage: withEnvVars("key1", "value1", "key2", "value2")
func withEnvVars(envVars ...string) envVarHelper {
	var ret envVarHelper

	for i := 0; i < len(envVars); i += 2 {
		if i+1 >= len(envVars) {
			panic("withEnvVars called with odd number of strings")
		}
		ret.envVars = append(ret.envVars, envVar{envVars[i], envVars[i+1]})
	}

	return ret
}

// docker build helper
type dbHelper struct {
	image    imageHelper
	matchers []interface{}
}

func db(img imageHelper, opts ...interface{}) dbHelper {
	return dbHelper{image: img, matchers: opts}
}

// custom build helper
type cbHelper struct {
	image    imageHelper
	matchers []interface{}
}

func cb(img imageHelper, opts ...interface{}) cbHelper {
	return cbHelper{img, opts}
}

type entrypointHelper struct {
	cmd model.Cmd
}

func entrypoint(command model.Cmd) entrypointHelper {
	return entrypointHelper{command}
}

type cmdHelper struct {
	cmd model.Cmd
}

func cmd(cmd string, dir string) cmdHelper {
	return cmdHelper{cmd: model.ToHostCmdInDir(cmd, dir)}
}

type tagHelper struct {
	tag string
}

func tag(tag string) tagHelper {
	return tagHelper{tag}
}

type depsHelper struct {
	deps []string
}

func deps(deps ...string) depsHelper {
	return depsHelper{deps}
}

type disablePushHelper struct {
	disabled bool
}

func disablePush(disable bool) disablePushHelper {
	return disablePushHelper{disable}
}

type updateCmdHelper struct {
	cmd model.Cmd
}

func updateCmd(dir string, cmd string, env []string) updateCmdHelper {
	return updateCmdHelper{cmd: model.ToHostCmdInDirWithEnv(cmd, dir, env)}
}

func updateCmdArray(dir string, argv []string, env []string) updateCmdHelper {
	return updateCmdHelper{cmd: model.Cmd{Argv: argv, Dir: dir, Env: env}}
}

type serveCmdHelper struct {
	cmd model.Cmd
}

func serveCmd(dir string, cmd string, env []string) serveCmdHelper {
	return serveCmdHelper{cmd: model.ToHostCmdInDirWithEnv(cmd, dir, env)}
}

func serveCmdArray(dir string, argv []string, env []string) serveCmdHelper {
	return serveCmdHelper{model.Cmd{Argv: argv, Dir: dir, Env: env}}
}

type readinessProbeHelper struct {
	probeSpec *v1alpha1.Probe
}

type localTargetHelper struct {
	matchers []interface{}
}

func localTarget(opts ...interface{}) localTargetHelper {
	return localTargetHelper{matchers: opts}
}

// useful scenarios to setup

// foo just has one image and one yaml
func (f *fixture) setupFoo() {
	f.dockerfile("foo/Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.gitInit("")
}

// bar just has one image and one yaml
func (f *fixture) setupFooAndBar() {
	f.dockerfile("foo/Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))

	f.dockerfile("bar/Dockerfile")
	f.yaml("bar.yaml", deployment("bar", image("gcr.io/bar")))

	f.gitInit("")
}

// expand has 4 images, a-d, and a yaml with all of it
func (f *fixture) setupExpand() {
	f.dockerfile("a/Dockerfile")
	f.dockerfile("b/Dockerfile")
	f.dockerfile("c/Dockerfile")
	f.dockerfile("d/Dockerfile")

	f.yaml("all.yaml",
		deployment("a", image("gcr.io/a")),
		deployment("b", image("gcr.io/b")),
		deployment("c", image("gcr.io/c")),
		deployment("d", image("gcr.io/d")),
	)

	f.gitInit("")
}

func (f *fixture) setupHelm() {
	f.file("helm/Chart.yaml", chartYAML)
	f.file("helm/values.yaml", valuesYAML)
	f.file("dev/helm/values-dev.yaml", valuesDevYAML) // make sure we can pull in a values.yaml file from outside chart dir

	f.file("helm/templates/_helpers.tpl", helpersTPL)
	f.file("helm/templates/deployment.yaml", deploymentYAML)
	f.file("helm/templates/ingress.yaml", ingressYAML)
	f.file("helm/templates/service.yaml", serviceYAML)
	f.file("helm/templates/namespace.yaml", namespaceYAML)
}

func (f *fixture) setupHelmWithRequirements() {
	f.setupHelm()

	nginxIngressChartPath := testdata.NginxIngressChartPath()
	f.CopyFile(nginxIngressChartPath, filepath.Join("helm/charts", filepath.Base(nginxIngressChartPath)))
}

func (f *fixture) setupHelmWithTest() {
	f.setupHelm()
	f.file("helm/templates/tests/test-mariadb-connection.yaml", helmTestYAML)
}

func (f *fixture) setupExtraPodSelectors(s string) {
	f.setupFoo()

	tiltfile := fmt.Sprintf(`

docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_resource('foo', extra_pod_selectors=%s)
`, s)

	f.file("Tiltfile", tiltfile)
}

func (f *fixture) setupCRD() {
	f.file("crd.yaml", `apiVersion: fission.io/v1
kind: Environment
metadata:
  name: mycrd
spec:
  builder:
    command: build
    image: test/mycrd-builder
  poolsize: 1
  runtime:
    image: test/mycrd-env`)
}

func overwriteSelectorsForService(entity *k8s.K8sEntity, labels map[string]string) error {
	svc, ok := entity.Obj.(*v1.Service)
	if !ok {
		return fmt.Errorf("don't know how to set selectors for %T", entity.Obj)
	}
	svc.Spec.Selector = labels
	return nil
}

func (f *fixture) SingleAnalyticsEvent(name string) analytics.CountEvent {
	var ret analytics.CountEvent
	for _, ce := range f.an.Counts {
		if ce.Name == name {
			require.Equalf(f.t, "", ret.Name, "two count events named %s", name)
			ret = ce
		}
	}
	require.NotEqualf(f.t, "", ret.Name, "no count event named %s", name)

	return ret
}

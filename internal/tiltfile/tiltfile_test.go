package tiltfile

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

func tempFile(content string) string {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		log.Fatal(err)
	}
	_, err = f.WriteString(content)
	if err != nil {
		log.Fatal(err)
	}

	return f.Name()
}

func TestSyntax(t *testing.T) {
	file := tempFile(`
def hello():
  a = lambda: print("hello")
  def b():
    a()

  b()

hello()
`)
	defer os.Remove(file)

	out := bytes.NewBuffer(nil)
	_, err := Load(file, out)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	s := out.String()
	expected := "hello\n"
	assert.Equal(t, expected, s)
}

func TestGetManifestConfig(t *testing.T) {
	dockerfile := tempFile("docker text")
	file := tempFile(
		fmt.Sprintf(`def blorgly():
  image = build_docker_image("%v", "docker-tag", "the entrypoint")
  image.add(local_git_repo('.'), '/mount_points/1')
  print(image.file_name)
  image.run("go install github.com/windmilleng/blorgly-frontend/server/...")
  image.run("echo hi")
  return k8s_service("yaaaaaaaaml", image)
`, dockerfile))
	defer os.Remove(file)
	defer os.Remove(dockerfile)

	tiltconfig, err := Load(file, os.Stdout)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	manifestConfig, err := tiltconfig.GetManifestConfigs("blorgly")
	if err != nil {
		t.Fatal("getting manifest config:", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal("couldn't get working directory:", err)
	}

	manifest := manifestConfig[0]
	assert.Equal(t, "docker text", manifest.DockerfileText)
	assert.Equal(t, "docker.io/library/docker-tag", manifest.DockerfileTag.String())
	assert.Equal(t, "yaaaaaaaaml", manifest.K8sYaml)
	assert.Equal(t, 1, len(manifest.Mounts), "number of mounts")
	assert.Equal(t, "/mount_points/1", manifest.Mounts[0].ContainerPath)
	assert.Equal(t, wd, manifest.Mounts[0].LocalPath, "mount path")
	assert.Equal(t, 2, len(manifest.Steps), "number of steps")
	assert.Equal(t, []string{"sh", "-c", "go install github.com/windmilleng/blorgly-frontend/server/..."}, manifest.Steps[0].Cmd.Argv, "first step")
	assert.Equal(t, []string{"sh", "-c", "echo hi"}, manifest.Steps[1].Cmd.Argv, "second step")
	assert.Equal(t, []string{"sh", "-c", "the entrypoint"}, manifest.Entrypoint.Argv)
}

func TestOldMountSyntax(t *testing.T) {
	dockerfile := tempFile("docker text")
	file := tempFile(
		fmt.Sprintf(`def blorgly():
  image = build_docker_image("%v", "docker-tag", "the entrypoint")
  image.add('/mount_points/1', local_git_repo('.'))
  print(image.file_name)
  image.run("go install github.com/windmilleng/blorgly-frontend/server/...")
  image.run("echo hi")
  return k8s_service("yaaaaaaaaml", image)
`, dockerfile))
	defer os.Remove(file)
	defer os.Remove(dockerfile)

	tiltconfig, err := Load(file, os.Stdout)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	_, err = tiltconfig.GetManifestConfigs("blorgly")
	if err == nil {
		t.Fatal("manifest config should have errored, but it didn't")
	}

	if !strings.Contains(err.Error(), oldMountSyntaxError) {
		t.Errorf("Expected error message to contain %s, got %v", oldMountSyntaxError, err)
	}

}

func TestGetManifestConfigMissingDockerFile(t *testing.T) {
	file := tempFile(
		fmt.Sprintf(`def blorgly():
  image = build_docker_image("asfaergiuhaeriguhaergiu", "docker-tag", "the entrypoint")
  print(image.file_name)
  image.run("go install github.com/windmilleng/blorgly-frontend/server/...")
  image.run("echo hi")
  return k8s_service("yaaaaaaaaml", image)
`))
	defer os.Remove(file)

	tiltconfig, err := Load(file, os.Stdout)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	_, err = tiltconfig.GetManifestConfigs("blorgly")
	if assert.NotNil(t, err, "expected error from missing dockerfile") {
		for _, s := range []string{"asfaergiuhaeriguhaergiu", "no such file or directory"} {
			assert.True(t, strings.Contains(err.Error(), s),
				"expected string '%s' not found in error: %v", s, err)
		}
	}
}

func TestLoadFunctions(t *testing.T) {
	file := tempFile(
		`def blorgly():
  return "blorgly"

def blorgly_backend():
  return "blorgly_backend"

def blorgly_frontend():
  return "blorgly_frontend"
`)
	defer os.Remove(file)

	tiltConfig, err := Load(file, os.Stdout)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	for _, s := range []string{"blorgly", "blorgly_backend", "blorgly_frontend"} {
		assert.Contains(t, tiltConfig.globals, s)
	}
}

func TestCompositeFunction(t *testing.T) {
	dockerfile := tempFile("docker text")
	file := tempFile(
		fmt.Sprintf(`def blorgly():
  return composite_service([blorgly_backend, blorgly_frontend])

def blorgly_backend():
    image = build_docker_image("%v", "docker-tag", "the entrypoint")
    print(image.file_name)
    image.run("go install github.com/windmilleng/blorgly-frontend/server/...")
    image.run("echo hi")
    return k8s_service("yaml", image)

def blorgly_frontend():
  image = build_docker_image("%v", "docker-tag", "the entrypoint")
  print(image.file_name)
  image.run("go install github.com/windmilleng/blorgly-frontend/server/...")
  image.run("echo hi")
  return k8s_service("yaaaaaaaaml", image)
`, dockerfile, dockerfile))
	defer os.Remove(file)
	defer os.Remove(dockerfile)

	tiltConfig, err := Load(file, os.Stdout)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	manifestConfig, err := tiltConfig.GetManifestConfigs("blorgly")
	if err != nil {
		t.Fatal("getting manifest config:", err)
	}

	assert.Equal(t, "blorgly_backend", manifestConfig[0].Name.String())
	assert.Equal(t, "blorgly_frontend", manifestConfig[1].Name.String())
}

func TestGetManifestConfigUndefined(t *testing.T) {
	file := tempFile(
		`def blorgly():
  return "yaaaaaaaml"
`)
	defer os.Remove(file)

	tiltConfig, err := Load(file, os.Stdout)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	_, err = tiltConfig.GetManifestConfigs("blorgly2")
	if err == nil {
		t.Fatal("expected error b/c of undefined manifest config:")
	}

	for _, s := range []string{"does not define", "blorgly2"} {
		assert.True(t, strings.Contains(err.Error(), s))
	}
}

func TestGetManifestConfigNonFunction(t *testing.T) {
	file := tempFile("blorgly2 = 3")
	defer os.Remove(file)

	tiltConfig, err := Load(file, os.Stdout)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	_, err = tiltConfig.GetManifestConfigs("blorgly2")
	if assert.NotNil(t, err, "GetManifestConfigs did not return an error") {
		for _, s := range []string{"blorgly2", "function", "int"} {
			assert.True(t, strings.Contains(err.Error(), s))
		}
	}
}

func TestGetManifestConfigTakesArgs(t *testing.T) {
	file := tempFile(
		`def blorgly2(x):
			return "foo"
`)
	defer os.Remove(file)

	tiltConfig, err := Load(file, os.Stdout)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	_, err = tiltConfig.GetManifestConfigs("blorgly2")
	if assert.NotNil(t, err, "GetManifestConfigs did not return an error") {
		for _, s := range []string{"blorgly2", "0 arguments"} {
			assert.True(t, strings.Contains(err.Error(), s))
		}
	}
}

func TestGetManifestConfigRaisesError(t *testing.T) {
	file := tempFile(
		`def blorgly2():
			"foo"[10]`) // index out of range
	defer os.Remove(file)

	tiltConfig, err := Load(file, os.Stdout)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	_, err = tiltConfig.GetManifestConfigs("blorgly2")
	if assert.NotNil(t, err, "GetManifestConfigs did not return an error") {
		for _, s := range []string{"blorgly2", "string index", "out of range"} {
			assert.True(t, strings.Contains(err.Error(), s), "error message '%V' did not contain '%V'", err.Error(), s)
		}
	}
}

func TestGetManifestConfigReturnsWrongType(t *testing.T) {
	file := tempFile(
		`def blorgly2():
			return "foo"`) // index out of range
	defer os.Remove(file)

	tiltConfig, err := Load(file, os.Stdout)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	_, err = tiltConfig.GetManifestConfigs("blorgly2")
	if assert.NotNil(t, err, "GetManifestConfigs did not return an error") {
		for _, s := range []string{"blorgly2", "string", "k8s_service"} {
			assert.True(t, strings.Contains(err.Error(), s), "error message '%V' did not contain '%V'", err.Error(), s)
		}
	}
}

func TestGetManifestConfigLocalReturnsNon0(t *testing.T) {
	file := tempFile(
		`def blorgly2():
			local('echo "foo" "bar" && echo "baz" "quu" >&2 && exit 1')`) // index out of range
	defer os.Remove(file)

	tiltConfig, err := Load(file, os.Stdout)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	_, err = tiltConfig.GetManifestConfigs("blorgly2")
	if assert.NotNil(t, err, "GetManifestConfigs did not return an error") {
		// "foo bar" and "baz quu" are separated above so that the match below only matches the strings in the output,
		// not in the command
		for _, s := range []string{"blorgly2", "exit status 1", "foo bar", "baz quu"} {
			assert.True(t, strings.Contains(err.Error(), s), "error message '%v' did not contain '%v'", err.Error(), s)
		}
	}
}

func TestGetManifestConfigWithLocalCmd(t *testing.T) {
	dockerfile := tempFile("docker text")
	file := tempFile(
		fmt.Sprintf(`def blorgly():
  image = build_docker_image("%v", "docker-tag", "the entrypoint")
  print(image.file_name)
  image.run("go install github.com/windmilleng/blorgly-frontend/server/...")
  image.run("echo hi")
  yaml = local('echo yaaaaaaaaml')
  return k8s_service(yaml, image)
`, dockerfile))
	defer os.Remove(file)
	defer os.Remove(dockerfile)

	tiltconfig, err := Load(file, os.Stdout)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	manifestConfig, err := tiltconfig.GetManifestConfigs("blorgly")
	if err != nil {
		t.Fatal("getting manifest config:", err)
	}

	manifest := manifestConfig[0]
	assert.Equal(t, "docker text", manifest.DockerfileText)
	assert.Equal(t, "docker.io/library/docker-tag", manifest.DockerfileTag.String())
	assert.Equal(t, "yaaaaaaaaml\n", manifest.K8sYaml)
	assert.Equal(t, 2, len(manifest.Steps))
	assert.Equal(t, []string{"sh", "-c", "go install github.com/windmilleng/blorgly-frontend/server/..."}, manifest.Steps[0].Cmd.Argv)
	assert.Equal(t, []string{"sh", "-c", "echo hi"}, manifest.Steps[1].Cmd.Argv)
	assert.Equal(t, []string{"sh", "-c", "the entrypoint"}, manifest.Entrypoint.Argv)
}

func TestRunTrigger(t *testing.T) {
	td := tempdir.NewTempDirFixture(t)
	defer td.TearDown()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWD)
	err = os.Chdir(td.Path())
	if err != nil {
		t.Fatal(err)
	}
	dockerfile := tempFile("docker text")
	file := tempFile(
		fmt.Sprintf(`def yarnly():
  image = build_docker_image("%v", "docker-tag", "the entrypoint")
  image.add(local_git_repo('.'), '/mount_points/1')
  image.run('yarn install', trigger='package.json')
  image.run('npm install', trigger=['package.json', 'yarn.lock'])
  image.run('echo hi')
  return k8s_service("yaaaaaaaaml", image)
`, dockerfile))
	defer os.Remove(file)
	defer os.Remove(dockerfile)

	tiltconfig, err := Load(file, os.Stdout)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	manifests, err := tiltconfig.GetManifestConfigs("yarnly")
	if err != nil {
		t.Fatal("getting manifest config:", err)
	}

	assert.Equal(t, len(manifests), 1)

	step0 := manifests[0].Steps[0]
	step1 := manifests[0].Steps[1]
	step2 := manifests[0].Steps[2]
	assert.Equal(
		t,
		step0.Cmd,
		model.Cmd{
			Argv: []string{"sh", "-c", "yarn install"},
		},
	)
	packagePath := td.JoinPath("package.json")
	matches, err := step0.Trigger.Matches(packagePath, false)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, matches)

	assert.Equal(
		t,
		step1.Cmd,
		model.Cmd{
			Argv: []string{"sh", "-c", "npm install"},
		},
	)
	matches, err = step1.Trigger.Matches(packagePath, false)
	yarnLockPath := td.JoinPath("yarn.lock")
	matches, err = step1.Trigger.Matches(yarnLockPath, false)
	assert.True(t, matches)

	randomPath := td.JoinPath("foo")
	matches, err = step1.Trigger.Matches(randomPath, false)
	assert.False(t, matches)

	assert.Equal(
		t,
		step2.Cmd,
		model.Cmd{
			Argv: []string{"sh", "-c", "echo hi"},
		},
	)

	assert.Nil(t, step2.Trigger)
}

func TestInvalidDockerTag(t *testing.T) {
	dockerfile := tempFile("docker text")
	file := tempFile(
		fmt.Sprintf(`def blorgly():
  image = build_docker_image(%q, "**invalid**", "the entrypoint")
  return k8s_service("yaaaaaaaaml", image)
`, dockerfile))
	defer os.Remove(file)
	defer os.Remove(dockerfile)
	tiltconfig, err := Load(file, os.Stdout)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}
	_, err = tiltconfig.GetManifestConfigs("blorgly")
	msg := "invalid reference format"
	if err == nil || !strings.Contains(err.Error(), msg) {
		t.Errorf("Expected error message to contain %v, got %v", msg, err)
	}
}

func TestEntrypointIsOptional(t *testing.T) {
	dockerfile := tempFile(`FROM alpine
ENTRYPOINT echo hi`)
	file := tempFile(
		fmt.Sprintf(`def blorgly():
  image = build_docker_image(%q, "docker-tag")
  return k8s_service("yaaaaaaaaml", image)
`, dockerfile))
	defer os.Remove(file)
	defer os.Remove(dockerfile)
	tiltconfig, err := Load(file, os.Stdout)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}
	manifests, err := tiltconfig.GetManifestConfigs("blorgly")
	if err != nil {
		t.Fatal(err)
	}
	manifest := manifests[0]
	// TODO(dmiller) is this right?
	assert.Equal(t, []string{"sh", "-c", ""}, manifest.Entrypoint.Argv)
}

func TestAddMissingDir(t *testing.T) {
	dockerfile := tempFile(`FROM alpine`)
	file := tempFile(
		fmt.Sprintf(`def blorgly():
  image = build_docker_image(%q, "docker-tag")
  image.add(local_git_repo('./garbage'), '/garbage')
  return k8s_service("yaaaaaaaaml", image)
`, dockerfile))
	defer os.Remove(file)
	defer os.Remove(dockerfile)

	c, err := Load(file, os.Stdout)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}
	_, err = c.GetManifestConfigs("blorgly")
	expected := "Reading path ./garbage"
	if err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("expected error message %q, actual: %v", expected, err)
	}
}
func TestReadFile(t *testing.T) {
	dockerfile := tempFile("docker text")
	fileToRead := tempFile("hello world")
	program := fmt.Sprintf(`def blorgly():
	yaml = read_file(%q)
	image = build_docker_image("%v", "docker-tag", "the entrypoint")
	return k8s_service(yaml, image)
`, fileToRead, dockerfile)
	file := tempFile(program)
	defer os.Remove(file)

	tiltConfig, err := Load(file, os.Stdout)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	s, err := tiltConfig.GetManifestConfigs("blorgly")

	assert.NotNil(t, s[0])
	assert.Equal(t, s[0].K8sYaml, "hello world")
}

func TestRepoPath(t *testing.T) {
	dockerfile := tempFile("docker text")
	fileToRead := tempFile("hello world")
	program := fmt.Sprintf(`def blorgly():
	repo = local_git_repo('.')
	print(repo.path('subpath'))
	yaml = read_file(%q)
	image = build_docker_image("%v", "docker-tag", repo.path('subpath'))
	return k8s_service(yaml, image)
`, fileToRead, dockerfile)
	file := tempFile(program)
	defer os.Remove(file)

	tiltConfig, err := Load(file, os.Stdout)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	manifestConfig, err := tiltConfig.GetManifestConfigs("blorgly")
	if err != nil {
		t.Fatal(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	manifest := manifestConfig[0]
	assert.Equal(t, []string{"sh", "-c", filepath.Join(wd, "subpath")}, manifest.Entrypoint.Argv)
}

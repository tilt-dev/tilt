package tiltfile

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/ignore"
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

type gitRepoFixture struct {
	*tempdir.TempDirFixture
	oldWD string
}

func newGitRepoFixture(t *testing.T) *gitRepoFixture {
	td := tempdir.NewTempDirFixture(t)
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chdir(td.Path())
	if err != nil {
		t.Fatal(err)
	}
	err = os.Mkdir(".git", os.FileMode(0777))
	if err != nil {
		t.Fatal(err)
	}
	return &gitRepoFixture{
		TempDirFixture: td,
		oldWD:          oldWD,
	}
}

func (f *gitRepoFixture) LoadManifest(name string) model.Manifest {
	tiltconfig, err := Load(f.JoinPath("Tiltfile"), os.Stdout)
	if err != nil {
		f.T().Fatal("loading tiltconfig:", err)
	}

	manifests, err := tiltconfig.GetManifestConfigs(name)
	if err != nil {
		f.T().Fatal("getting manifest config:", err)
	}

	if len(manifests) != 1 {
		f.T().Fatalf("expected 1 manifest, actual: %d", len(manifests))
	}

	return manifests[0]
}

func (f *gitRepoFixture) LoadManifestForError(name string) error {
	tiltconfig, err := Load(f.JoinPath("Tiltfile"), os.Stdout)
	if err != nil {
		f.T().Fatal("loading tiltconfig:", err)
	}

	_, err = tiltconfig.GetManifestConfigs(name)
	if err == nil {
		f.T().Fatal("Expected manifest load error")
	}
	return err
}

func (f *gitRepoFixture) FiltersPath(manifest model.Manifest, path string, isDir bool) bool {
	matches, err := ignore.CreateFilter(manifest).Matches(f.JoinPath(path), isDir)
	if err != nil {
		f.T().Fatal(err)
	}
	return matches
}

func (f *gitRepoFixture) TearDown() {
	f.TempDirFixture.TearDown()
	_ = os.Chdir(f.oldWD)
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
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Dockerfile.base", "docker text")
	f.WriteFile("Tiltfile", `def blorgly():
  start_fast_build("Dockerfile.base", "docker-tag", "the entrypoint")
  add(local_git_repo('.'), '/mount_points/1')
  run("go install github.com/windmilleng/blorgly-frontend/server/...")
  run("echo hi")
  image = stop_build()
  return k8s_service("yaaaaaaaaml", image)
`)

	manifest := f.LoadManifest("blorgly")
	assert.Equal(t, "docker text", manifest.BaseDockerfile)
	assert.Equal(t, "docker.io/library/docker-tag", manifest.DockerRef.String())
	assert.Equal(t, "yaaaaaaaaml", manifest.K8sYaml)
	assert.Equal(t, 1, len(manifest.Mounts), "number of mounts")
	assert.Equal(t, "/mount_points/1", manifest.Mounts[0].ContainerPath)
	assert.Equal(t, f.Path(), manifest.Mounts[0].LocalPath, "mount path")
	assert.Equal(t, 2, len(manifest.Steps), "number of steps")
	assert.Equal(t, []string{"sh", "-c", "go install github.com/windmilleng/blorgly-frontend/server/..."}, manifest.Steps[0].Cmd.Argv, "first step")
	assert.Equal(t, []string{"sh", "-c", "echo hi"}, manifest.Steps[1].Cmd.Argv, "second step")
	assert.Equal(t, []string{"sh", "-c", "the entrypoint"}, manifest.Entrypoint.Argv)
}

func TestOldMountSyntax(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Dockerfile.base", "docker text")
	f.WriteFile("Tiltfile", `def blorgly():
  start_fast_build("Dockerfile.base", "docker-tag", "the entrypoint")
  add('/mount_points/1', local_git_repo('.'))
  print(image.file_name)
  run("go install github.com/windmilleng/blorgly-frontend/server/...")
  run("echo hi")
  image = stop_build()
  return k8s_service("yaaaaaaaaml", image)
`)

	err := f.LoadManifestForError("blorgly")
	if !strings.Contains(err.Error(), oldMountSyntaxError) {
		t.Errorf("Expected error message to contain %s, got %v", oldMountSyntaxError, err)
	}
}

func TestStopBuildBeforeStartBuild(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Dockerfile.base", "docker text")
	f.WriteFile("Tiltfile", `def blorgly():
  return stop_build()
`)

	err := f.LoadManifestForError("blorgly")
	if !strings.Contains(err.Error(), noActiveBuildError) {
		t.Errorf("Expected error message to contain %s, got %v", noActiveBuildError, err)
	}
}

func TestGetManifestConfigMissingDockerFile(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `def blorgly():
  start_fast_build("asfaergiuhaeriguhaergiu", "docker-tag", "the entrypoint")
  run("go install github.com/windmilleng/blorgly-frontend/server/...")
  run("echo hi")
  image = stop_build()
  return k8s_service("yaaaaaaaaml", image)
`)

	err := f.LoadManifestForError("blorgly")
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
  start_fast_build("%v", "docker-tag", "the entrypoint")
  add(local_git_repo('.'), '/mount_points/1')
  run("go install github.com/windmilleng/blorgly-frontend/server/...")
  run("echo hi")
  image = stop_build()
  return k8s_service("yaml", image)

def blorgly_frontend():
  start_fast_build("%v", "docker-tag", "the entrypoint")
  add(local_git_repo('.'), '/mount_points/2')
  run("go install github.com/windmilleng/blorgly-frontend/server/...")
  run("echo hi")
  image = stop_build()
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
	assert.Equal(t, 1, len(manifestConfig[0].Repos))
	assert.Equal(t, "", manifestConfig[0].Repos[0].DockerignoreContents)
	assert.Equal(t, "", manifestConfig[0].Repos[0].GitignoreContents)
	assert.Equal(t, "blorgly_frontend", manifestConfig[1].Name.String())
	assert.Equal(t, 1, len(manifestConfig[1].Repos))
	assert.Equal(t, "", manifestConfig[1].Repos[0].DockerignoreContents)
	assert.Equal(t, "", manifestConfig[1].Repos[0].GitignoreContents)
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
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Dockerfile.base", "docker text")
	f.WriteFile("Tiltfile", `def blorgly():
  start_fast_build("Dockerfile.base", "docker-tag", "the entrypoint")
  run("go install github.com/windmilleng/blorgly-frontend/server/...")
  run("echo hi")
  yaml = local('echo yaaaaaaaaml')
  image = stop_build()
  return k8s_service(yaml, image)
`)

	manifest := f.LoadManifest("blorgly")
	assert.Equal(t, "docker text", manifest.BaseDockerfile)
	assert.Equal(t, "docker.io/library/docker-tag", manifest.DockerRef.String())
	assert.Equal(t, "yaaaaaaaaml\n", manifest.K8sYaml)
	assert.Equal(t, 2, len(manifest.Steps))
	assert.Equal(t, []string{"sh", "-c", "go install github.com/windmilleng/blorgly-frontend/server/..."}, manifest.Steps[0].Cmd.Argv)
	assert.Equal(t, []string{"sh", "-c", "echo hi"}, manifest.Steps[1].Cmd.Argv)
	assert.Equal(t, []string{"sh", "-c", "the entrypoint"}, manifest.Entrypoint.Argv)
}

func TestRunTrigger(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Dockerfile.base", "docker text")
	f.WriteFile("Tiltfile", `def yarnly():
  start_fast_build("Dockerfile.base", "docker-tag", "the entrypoint")
  add(local_git_repo('.'), '/mount_points/1')
  run('yarn install', trigger='package.json')
  run('npm install', trigger=['package.json', 'yarn.lock'])
  run('echo hi')
  image = stop_build()
  return k8s_service("yaaaaaaaaml", image)
`)

	manifest := f.LoadManifest("yarnly")
	step0 := manifest.Steps[0]
	step1 := manifest.Steps[1]
	step2 := manifest.Steps[2]
	assert.Equal(
		t,
		step0.Cmd,
		model.Cmd{
			Argv: []string{"sh", "-c", "yarn install"},
		},
	)
	packagePath := f.JoinPath("package.json")
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
	yarnLockPath := f.JoinPath("yarn.lock")
	matches, err = step1.Trigger.Matches(yarnLockPath, false)
	assert.True(t, matches)

	randomPath := f.JoinPath("foo")
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
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Dockerfile.base", "docker text")
	f.WriteFile("Tiltfile", `def blorgly():
  start_fast_build("Dockerfile.base", "**invalid**", "the entrypoint")
  image = stop_build()
  return k8s_service("yaaaaaaaaml", image)
`)
	err := f.LoadManifestForError("blorgly")
	msg := "invalid reference format"
	if err == nil || !strings.Contains(err.Error(), msg) {
		t.Errorf("Expected error message to contain %v, got %v", msg, err)
	}
}

func TestEntrypointIsOptional(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Dockerfile.base", `FROM alpine
ENTRYPOINT echo hi`)
	f.WriteFile("Tiltfile", `def blorgly():
  start_fast_build("Dockerfile.base", "docker-tag")
  image = stop_build()
  return k8s_service("yaaaaaaaaml", image)
`)

	manifest := f.LoadManifest("blorgly")
	assert.Equal(t, []string(nil), manifest.Entrypoint.Argv)
	assert.True(t, manifest.Entrypoint.Empty())
}

func TestAddMissingDir(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Dockerfile.base", "FROM alpine")
	f.WriteFile("Tiltfile", `def blorgly():
  start_fast_build("Dockerfile.base", "docker-tag")
  add(local_git_repo('./garbage'), '/garbage')
  image = stop_build()
  return k8s_service("yaaaaaaaaml", image)
`)

	err := f.LoadManifestForError("blorgly")
	expected := "Reading path ./garbage"
	if err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("expected error message %q, actual: %v", expected, err)
	}
}

func TestReadFile(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Dockerfile.base", "docker text")
	f.WriteFile("a.txt", "hello world")
	f.WriteFile("Tiltfile", `def blorgly():
  yaml = read_file("a.txt")
  start_fast_build("Dockerfile.base", "docker-tag", "the entrypoint")
  image = stop_build()
  return k8s_service(yaml, image)
`)

	manifest := f.LoadManifest("blorgly")
	assert.Equal(t, manifest.K8sYaml, "hello world")
}

func TestConfigMatcherWithFastBuild(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()
	f.WriteFile("Dockerfile.base", "docker text")
	f.WriteFile("a.txt", "a")
	f.WriteFile("b.txt", "b")
	f.WriteFile("Tiltfile", `def blorgly():
  yaml = read_file("a.txt")
  start_fast_build("Dockerfile.base", "docker-tag", "the entrypoint")
  image = stop_build()
  return k8s_service(yaml, image)
`)
	manifest := f.LoadManifest("blorgly")
	matches := func(p string) bool {
		matcher, err := manifest.ConfigMatcher()
		if err != nil {
			t.Fatal(err)
		}
		ok, err := matcher.Matches(p, false)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	assert.True(t, matches(f.JoinPath("Dockerfile.base")))
	assert.True(t, matches(f.JoinPath("Tiltfile")))
	assert.True(t, matches(f.JoinPath("a.txt")))
	assert.False(t, matches(f.JoinPath("b.txt")))
}

func TestConfigMatcherWithStaticBuild(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()
	f.WriteFile("Dockerfile", "dockerfile text")
	f.WriteFile("Tiltfile", `def blorgly():
  yaml = local('echo yaaaaaaaaml')
  return k8s_service(yaml, static_build("Dockerfile", "docker-tag"))`)
	manifest := f.LoadManifest("blorgly")
	matches := func(p string) bool {
		matcher, err := manifest.ConfigMatcher()
		if err != nil {
			t.Fatal(err)
		}
		ok, err := matcher.Matches(p, false)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	assert.True(t, matches(f.JoinPath("Dockerfile")))
	assert.True(t, matches(f.JoinPath("Tiltfile")))
	assert.False(t, matches(f.JoinPath("b.txt")))
}

func TestRepoPath(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Dockerfile.base", "docker text")
	f.WriteFile("a.txt", "hello world")
	f.WriteFile("Tiltfile", `def blorgly():
  repo = local_git_repo('.')
  print(repo.path('subpath'))
  yaml = read_file("a.txt")
  start_fast_build("Dockerfile.base", "docker-tag", str(repo.path('subpath')))
  image = stop_build()
  return k8s_service(yaml, image)
`)

	manifest := f.LoadManifest("blorgly")
	assert.Equal(t, []string{"sh", "-c", f.JoinPath("subpath")}, manifest.Entrypoint.Argv)
}

func TestAddErorrsIfStringPassedInsteadOfRepoPath(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Dockerfile.base", "docker text")
	f.WriteFile("a.txt", "hello world")
	f.WriteFile("Tiltfile", `def blorgly():
  repo = local_git_repo('.')
  yaml = read_file("a.txt")
  start_fast_build("Dockerfile.base", "docker-tag", str(repo.path('subpath')))
  add("package.json", "/app/package.json")
  image = stop_build()
  return k8s_service(yaml, image)
`)

	err := f.LoadManifestForError("blorgly")
	expected := "invalid type for src. Got string want gitRepo OR localPath"
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("Expected %s to contain %s", err.Error(), expected)
	}
}

func TestAddOneFileByPath(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Dockerfile.base", "docker text")
	f.WriteFile("a.txt", "hello world")
	f.WriteFile("Tiltfile", `def blorgly():
  repo = local_git_repo('.')
  print(repo.path('subpath'))
  yaml = read_file("a.txt")
  start_fast_build("Dockerfile.base", "docker-tag", str(repo.path('subpath')))
  add(repo.path('package.json'), '/app/package.json')
  image = stop_build()
  return k8s_service(yaml, image)
`)

	manifest := f.LoadManifest("blorgly")
	assert.Equal(t, manifest.Mounts[0].LocalPath, f.JoinPath("package.json"))
	assert.Equal(t, manifest.Mounts[0].ContainerPath, "/app/package.json")
}

func TestFailsIfNotGitRepo(t *testing.T) {
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
		fmt.Sprintf(`def blorgly():
  start_fast_build("%v", "docker-tag", "the entrypoint")
  add(local_git_repo('.'), '/mount_points/1')
  run("go install github.com/windmilleng/blorgly-frontend/server/...")
  run("echo hi")
  image = stop_build()
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
		t.Error("Expected error")
	} else if !strings.Contains(err.Error(), "isn't a valid git repo") {
		t.Errorf("Expected error to be an invalid git repo error, got %s", err.Error())
	}
}

func TestReadsIgnoreFiles(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()
	f.WriteFile(".gitignore", "*.exe")
	f.WriteFile(".dockerignore", "node_modules")
	f.WriteFile("Dockerfile", "docker text")
	f.WriteFile("Tiltfile", `def blorgly():
  start_fast_build("Dockerfile", "docker-tag", "the entrypoint")
  add(local_git_repo('.'), '/mount_points/1')
  run("go install github.com/windmilleng/blorgly-frontend/server/...")
  run("echo hi")
  image = stop_build()
  return k8s_service("yaaaaaaaaml", image)
`)

	manifest := f.LoadManifest("blorgly")
	assert.Truef(t, f.FiltersPath(manifest, "Tiltfile", true), "Expected to filter Tiltfile")
	assert.True(t, f.FiltersPath(manifest, "cmd.exe", false), "Expected to filter cmd.exe")
	assert.True(t, f.FiltersPath(manifest, ".git", true), "Expected to filter .git")
	assert.True(t, f.FiltersPath(manifest, "node_modules", true), "Expected to filter node_modules")
	assert.False(t, f.FiltersPath(manifest, "a.txt", false), "Expected to filter a.txt")
}

func TestReadsIgnoreFilesMultipleGitRepos(t *testing.T) {
	f1 := newGitRepoFixture(t)
	defer f1.TearDown()

	f2 := newGitRepoFixture(t)
	defer f2.TearDown()

	f1.WriteFile(".gitignore", "*.exe")
	f1.WriteFile(".dockerignore", "node_modules")
	f2.WriteFile(".dockerignore", "*.txt")

	// We don't use the standard test setup because we want to test
	// external repos.
	dockerfile := tempFile("docker text")
	file := tempFile(
		fmt.Sprintf(`def blorgly():
  start_fast_build("%v", "docker-tag", "the entrypoint")
  add(local_git_repo('%s'), '/mount_points/1')
  add(local_git_repo('%s'), '/mount_points/2')
  run("go install github.com/windmilleng/blorgly-frontend/server/...")
  run("echo hi")
  image = stop_build()
  return k8s_service("yaaaaaaaaml", image)
`, dockerfile, f1.Path(), f2.Path()))
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
	if len(manifests) == 0 {
		t.Fatal("Expected at least 1 manifest, got 0")
	}

	manifest := manifests[0]

	assert.Truef(t, f1.FiltersPath(manifest, "cmd.exe", false), "Expected to match cmd.exe")
	assert.Truef(t, f1.FiltersPath(manifest, "node_modules", true), "Expected to match node_modules")
	assert.Truef(t, f1.FiltersPath(manifest, ".git", true), "Expected to match .git")
	assert.Falsef(t, f1.FiltersPath(manifest, "a.txt", false), "Expected to not match a.txt")
	assert.Falsef(t, f2.FiltersPath(manifest, "cmd.exe", false), "Expected to not much cmd.exe")
	assert.Falsef(t, f2.FiltersPath(manifest, "node_modules", true), "Expected to not match node_modules")
	assert.Truef(t, f2.FiltersPath(manifest, ".git", true), "Expected to match .git")
	assert.Truef(t, f2.FiltersPath(manifest, "a.txt", false), "Expected to match a.txt")
}

func TestBuildContextAddError(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()
	f.WriteFile("Dockerfile.base", "docker text")
	f.WriteFile("Tiltfile", `def blorgly():
  start_fast_build("Dockerfile.base", "docker-tag", "the entrypoint")
  add(local_git_repo('.'), '/mount_points/1')
  run("go install github.com/windmilleng/blorgly-frontend/server/...")
  run("echo hi")
  image = stop_build()
  add(local_git_repo('.'), '/mount_points/2')
  return k8s_service("yaaaaaaaaml", image)
`)

	err := f.LoadManifestForError("blorgly")
	expected := "error running 'blorgly': add called without a build context"
	if err.Error() != expected {
		t.Errorf("Expected %s to equal %s", err.Error(), expected)
	}
}

func TestBuildContextRunError(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()
	f.WriteFile("Dockerfile.base", "docker text")
	f.WriteFile("Tiltfile", `def blorgly():
  start_fast_build("Dockerfile.base", "docker-tag", "the entrypoint")
  add(local_git_repo('.'), '/mount_points/1')
  run("go install github.com/windmilleng/blorgly-frontend/server/...")
  run("echo hi")
  image = stop_build()
  run("echo hi2")
  return k8s_service("yaaaaaaaaml", image)
`)

	err := f.LoadManifestForError("blorgly")
	expected := "error running 'blorgly': run called without a build context"
	if err.Error() != expected {
		t.Errorf("Expected %s to equal %s", err.Error(), expected)
	}
}

func TestBuildContextStartTwice(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()
	f.WriteFile("Dockerfile.base", "docker text")
	f.WriteFile("Tiltfile", `def blorgly():
  start_fast_build("Dockerfile.base", "docker-tag", "the entrypoint")
  start_fast_build("Dockerfile.base", "docker-tag2", "new entrypoint")
  add(local_git_repo('.'), '/mount_points/1')
  run("go install github.com/windmilleng/blorgly-frontend/server/...")
  image = stop_build()
  return k8s_service("yaaaaaaaaml", image)
`)

	err := f.LoadManifestForError("blorgly")
	expected := "error running 'blorgly': tried to start a build context while another build context was already open"
	if err.Error() != expected {
		t.Errorf("Expected %s to equal %s", err.Error(), expected)
	}
}

func TestSlowBuildIsNotImplemented(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()
	f.WriteFile(".gitignore", "*.exe")
	f.WriteFile(".dockerignore", "node_modules")
	f.WriteFile("Dockerfile.base", "docker text")
	f.WriteFile("Tiltfile", `def blorgly():
  image = start_slow_build("Dockerfile.base", "docker-tag", "the entrypoint")
  image.add(local_git_repo('.'), '/mount_points/1')
  image.run("go install github.com/windmilleng/blorgly-frontend/server/...")
  image.run("echo hi")
  return k8s_service("yaaaaaaaaml", image)
`)
	err := f.LoadManifestForError("blorgly")
	expected := "error running 'blorgly': start_slow_build not implemented"
	if err.Error() != expected {
		t.Errorf("Expected %s to equal %s", err.Error(), expected)
	}
}

func TestStaticBuild(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Dockerfile", "dockerfile text")
	f.WriteFile("Tiltfile", `
def blorgly():
  yaml = local('echo yaaaaaaaaml')
  return k8s_service(yaml, static_build("Dockerfile", "docker-tag"))
`)

	manifest := f.LoadManifest("blorgly")
	assert.Equal(t, "dockerfile text", manifest.StaticDockerfile)
	assert.Equal(t, f.Path(), manifest.StaticBuildPath)
	assert.Equal(t, "docker.io/library/docker-tag", manifest.DockerRef.String())
}

func TestPortForward(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Dockerfile", "dockerfile text")
	f.WriteFile("Tiltfile", `
def blorgly():
  yaml = local('echo yaaaaaaaaml')
  s = k8s_service(yaml, static_build("Dockerfile", "docker-tag"))
  s.port_forward(8000)
  s.port_forward(8001, 443)
  return s
`)

	manifest := f.LoadManifest("blorgly")
	assert.Equal(t, []model.PortForward{
		{
			LocalPort:     8000,
			ContainerPort: 0,
		}, {
			LocalPort:     8001,
			ContainerPort: 443,
		},
	}, manifest.PortForwards)
}

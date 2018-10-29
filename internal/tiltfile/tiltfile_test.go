package tiltfile

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/windmilleng/tilt/internal/testutils/output"

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
	ctx   context.Context
	out   *bytes.Buffer
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

	out := new(bytes.Buffer)
	ctx := output.ForkedCtxForTest(out)
	return &gitRepoFixture{
		TempDirFixture: td,
		oldWD:          oldWD,
		ctx:            ctx,
		out:            out,
	}
}

func (f *gitRepoFixture) LoadManifests(names ...string) []model.Manifest {
	// It's important that this uses a relative path, because
	// that's how other places in Tilt call it. In the past, we've had
	// a lot of bugs that come up due to relative paths vs. absolute paths.
	tiltconfig, err := Load(f.ctx, FileName)
	if err != nil {
		f.T().Fatal("loading tiltconfig:", err)
	}

	manifests, _, err := tiltconfig.GetManifestConfigsAndGlobalYAML(names...)
	if err != nil {
		f.T().Fatal("getting manifest config:", err)
	}
	return manifests
}

func (f *gitRepoFixture) LoadManifest(name string) model.Manifest {
	manifests := f.LoadManifests(name)
	if len(manifests) != 1 {
		f.T().Fatalf("expected 1 manifest, actual: %d", len(manifests))
	}
	return manifests[0]
}

func (f *gitRepoFixture) LoadManifestForError(name string) error {
	tiltconfig, err := Load(f.ctx, f.JoinPath("Tiltfile"))
	if err != nil {
		f.T().Fatal("loading tiltconfig:", err)
	}

	_, _, err = tiltconfig.GetManifestConfigsAndGlobalYAML(name)
	if err == nil {
		f.T().Fatal("Expected manifest load error")
	}
	return err
}

func (f *gitRepoFixture) FiltersPath(manifest model.Manifest, path string, isDir bool) bool {
	matches, err := ignore.CreateBuildContextFilter(manifest).Matches(f.JoinPath(path), isDir)
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
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `
def hello():
  a = lambda: print("hello")
  def b():
    a()

  b()

hello()
`)
	_, err := Load(f.ctx, "Tiltfile")
	if err != nil {
		t.Fatal(err)
	}

	s := f.out.String()
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
	assert.Equal(t, "docker.io/library/docker-tag", manifest.DockerRef().String())
	assert.Equal(t, "yaaaaaaaaml", manifest.K8sYAML())
	assert.Equal(t, 1, len(manifest.Mounts), "number of mounts")
	assert.Equal(t, "/mount_points/1", manifest.Mounts[0].ContainerPath)
	assert.Equal(t, f.Path(), manifest.Mounts[0].LocalPath, "mount path")
	assert.Equal(t, 2, len(manifest.Steps), "number of steps")
	assert.Equal(t, []string{"sh", "-c", "go install github.com/windmilleng/blorgly-frontend/server/..."}, manifest.Steps[0].Cmd.Argv, "first step")
	assert.Equal(t, []string{"sh", "-c", "echo hi"}, manifest.Steps[1].Cmd.Argv, "second step")
	assert.Equal(t, []string{"sh", "-c", "the entrypoint"}, manifest.Entrypoint.Argv)
	assert.Equal(t, f.JoinPath("Tiltfile"), manifest.TiltFilename())
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
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), oldMountSyntaxError)
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
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), noActiveBuildError)
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
	if assert.Error(t, err) {
		for _, s := range []string{"asfaergiuhaeriguhaergiu", "no such file or directory"} {
			assert.Contains(t, err.Error(), s)
		}
	}
}

func TestCompositeFunction(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Dockerfile", "docker text")
	f.WriteFile("Tiltfile", `
def blorgly():
  return composite_service([blorgly_backend, blorgly_frontend])

def blorgly_backend():
  start_fast_build("Dockerfile", "docker-tag", "the entrypoint")
  add(local_git_repo('.'), '/mount_points/1')
  run("go install github.com/windmilleng/blorgly-frontend/server/...")
  run("echo hi")
  image = stop_build()
  return k8s_service("yaml", image)

def blorgly_frontend():
  start_fast_build("Dockerfile", "docker-tag", "the entrypoint")
  add(local_git_repo('.'), '/mount_points/2')
  run("go install github.com/windmilleng/blorgly-frontend/server/...")
  run("echo hi")
  image = stop_build()
  return k8s_service("yaaaaaaaaml", image)
`)

	manifests := f.LoadManifests("blorgly")
	assert.Equal(t, "blorgly_backend", manifests[0].Name.String())
	assert.Equal(t, 1, len(manifests[0].Repos))
	assert.Equal(t, "", manifests[0].Repos[0].DockerignoreContents)
	assert.Equal(t, "", manifests[0].Repos[0].GitignoreContents)
	assert.Equal(t, "blorgly_frontend", manifests[1].Name.String())
	assert.Equal(t, 1, len(manifests[1].Repos))
	assert.Equal(t, "", manifests[1].Repos[0].DockerignoreContents)
	assert.Equal(t, "", manifests[1].Repos[0].GitignoreContents)
}

func TestGetManifestConfigUndefined(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `def blorgly():
  return "yaaaaaaaml"
`)

	err := f.LoadManifestForError("blorgly2")

	for _, s := range []string{"does not define", "blorgly2"} {
		assert.Contains(t, err.Error(), s)
	}
}

func TestGetManifestConfigNonFunction(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", "blorgly2 = 3")

	err := f.LoadManifestForError("blorgly2")

	for _, s := range []string{"blorgly2", "function", "int"} {
		assert.Contains(t, err.Error(), s)
	}
}

func TestGetManifestConfigTakesArgs(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `def blorgly2(x):
      return "foo"
`)

	err := f.LoadManifestForError("blorgly2")

	for _, s := range []string{"blorgly2", "0 arguments"} {
		assert.Contains(t, err.Error(), s)
	}
}

func TestGetManifestConfigRaisesError(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `def blorgly2():
      "foo"[10]`) // index out of range

	err := f.LoadManifestForError("blorgly2")

	for _, s := range []string{"blorgly2", "string index", "out of range"} {
		assert.Contains(t, err.Error(), s)
	}
}

func TestGetManifestConfigReturnsWrongType(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `def blorgly2():
      return "foo"`)

	err := f.LoadManifestForError("blorgly2")

	for _, s := range []string{"blorgly2", "string", "k8s_service"} {
		assert.Contains(t, err.Error(), s)
	}
}

func TestGetManifestConfigLocalReturnsNon0(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `def blorgly2():
      local('echo "foo" "bar" && echo "baz" "quu" >&2 && exit 1')`)

	err := f.LoadManifestForError("blorgly2")

	// "foo bar" and "baz quu" are separated above so that the match below only matches the strings in the output,
	// not in the command
	for _, s := range []string{"blorgly2", "exit status 1", "foo bar", "baz quu"} {
		assert.Contains(t, err.Error(), s)
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
	assert.Equal(t, "docker.io/library/docker-tag", manifest.DockerRef().String())
	assert.Equal(t, "yaaaaaaaaml\n", manifest.K8sYAML())
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
	matcher, err := ignore.CreateStepMatcher(step0)
	if err != nil {
		t.Fatal(err)
	}
	matches, err := matcher.Matches(packagePath, false)
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
	matcher, err = ignore.CreateStepMatcher(step1)
	if err != nil {
		t.Fatal(err)
	}
	matches, err = matcher.Matches(packagePath, false)
	yarnLockPath := f.JoinPath("yarn.lock")
	matches, err = matcher.Matches(yarnLockPath, false)
	assert.True(t, matches)

	randomPath := f.JoinPath("foo")
	matches, err = matcher.Matches(randomPath, false)
	assert.False(t, matches)

	assert.Equal(
		t,
		step2.Cmd,
		model.Cmd{
			Argv: []string{"sh", "-c", "echo hi"},
		},
	)

	assert.Nil(t, step2.Triggers)
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
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "invalid reference format")
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
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "Reading path ./garbage")
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
	assert.Equal(t, "hello world", manifest.K8sYAML())
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

func TestAddRepoPath(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile(".gitignore", "*.txt")
	f.WriteFile("Dockerfile.base", "docker text")
	f.WriteFile("a.txt", "a")
	f.WriteFile("src/b.txt", "b")
	f.WriteFile("src/b/c.txt", "c")
	f.WriteFile("src/b/d.go", "d")
	f.WriteFile("Tiltfile", `def blorgly():
  repo = local_git_repo('.')
  start_fast_build("Dockerfile.base", "docker-tag")
  add(repo.path('src'), '/src')
  image = stop_build()
  return k8s_service("", image)
`)

	manifest := f.LoadManifest("blorgly")
	assert.True(t, f.FiltersPath(manifest, "src/b.txt", false), "Expected to filter b.txt")
	assert.True(t, f.FiltersPath(manifest, "src/b/c.txt", false), "Expected to filter c.txt")
	assert.False(t, f.FiltersPath(manifest, "src/b/d.go", false), "Expected not to filter d.txt")
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
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "invalid type for src. Got string want gitRepo OR localPath")
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
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()
	f.WriteFile("Dockerfile", "docker text")
	f.WriteFile("Tiltfile", `
def blorgly():
  start_fast_build("Dockerfile", "docker-tag", "the entrypoint")
  add(local_git_repo('.'), '/mount_points/1')
  run("go install github.com/windmilleng/blorgly-frontend/server/...")
  run("echo hi")
  image = stop_build()
  return k8s_service("yaaaaaaaaml", image)
`)

	ctx := output.CtxForTest()
	tiltconfig, err := Load(ctx, f.JoinPath("Tiltfile"))
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}
	_, _, err = tiltconfig.GetManifestConfigsAndGlobalYAML("blorgly")
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "isn't a valid git repo")
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

func TestReadsIgnoreFilesStaticBuild(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()
	f.WriteFile(".gitignore", "*.exe")
	f.WriteFile(".dockerignore", "node_modules")
	f.WriteFile("Dockerfile", "docker text")
	f.WriteFile("Tiltfile", `def blorgly():
  image = static_build("Dockerfile", "docker-tag")
  return k8s_service("yaaaaaaaaml", image)
`)

	manifest := f.LoadManifest("blorgly")
	assert.Truef(t, f.FiltersPath(manifest, "Tiltfile", true), "Expected to filter Tiltfile")
	assert.True(t, f.FiltersPath(manifest, "cmd.exe", false), "Expected to filter cmd.exe")
	assert.True(t, f.FiltersPath(manifest, ".git", true), "Expected to filter .git")
	assert.True(t, f.FiltersPath(manifest, "node_modules", true), "Expected to filter node_modules")
	assert.False(t, f.FiltersPath(manifest, "a.txt", false), "Expected to filter a.txt")
}

func TestReadsIgnoreFilesStaticBuildSubdir(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()
	f.WriteFile(".gitignore", "*.exe")
	f.WriteFile(".dockerignore", "node_modules")
	f.WriteFile("subdir/Dockerfile", "docker text")
	f.WriteFile("Tiltfile", `def blorgly():
  repo = local_git_repo(".")
  image = static_build(repo.path("subdir/Dockerfile"), "docker-tag", context=repo)
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

	// This needs to go last so it sets the working directory.
	fMain := newGitRepoFixture(t)
	defer fMain.TearDown()

	// We don't use the standard test setup because we want to test
	// external repos.
	fMain.WriteFile("Dockerfile", "docker text")
	fMain.WriteFile("Tiltfile",
		fmt.Sprintf(`def blorgly():
  start_fast_build("Dockerfile", "docker-tag", "the entrypoint")
  add(local_git_repo('%s'), '/mount_points/1')
  add(local_git_repo('%s'), '/mount_points/2')
  run("go install github.com/windmilleng/blorgly-frontend/server/...")
  run("echo hi")
  image = stop_build()
  return k8s_service("yaaaaaaaaml", image)
`, f1.Path(), f2.Path()))

	manifest := fMain.LoadManifest("blorgly")

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
	assert.Equal(t, "docker.io/library/docker-tag", manifest.DockerRef().String())
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
	}, manifest.PortForwards())
}

func TestSymlinkInPath(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("real/Dockerfile", "dockerfile text")
	f.WriteFile("real/Tiltfile", `
def blorgly():
  yaml = local('echo yaaaaaaaaml')
  start_fast_build("Dockerfile", "docker-tag")
  add(local_git_repo('.'), '/src')
  return k8s_service(yaml, stop_build())
`)
	_ = os.Mkdir(f.JoinPath("real", ".git"), os.FileMode(0777))
	_ = os.Symlink(f.JoinPath("real"), f.JoinPath("fake"))

	ctx := output.CtxForTest()
	tiltconfig, err := Load(ctx, filepath.Join("fake", FileName))
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	manifests, _, err := tiltconfig.GetManifestConfigsAndGlobalYAML("blorgly")
	if err != nil {
		t.Fatal("getting manifest config:", err)
	}
	manifest := manifests[0]

	// We used to have a bug where the mounts would use the fake path,
	// but the file watchers would emit the real path, which would
	// break systems where the directory had a symlink somewhere in the
	// ancestor tree.
	assert.Equal(t, f.JoinPath("real", FileName), manifest.TiltFilename())
	assert.Equal(t, f.JoinPath("real"), manifest.Mounts[0].LocalPath)
}

func TestKustomize(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()
	kustomizeFile := `# Example configuration for the webserver
# at https://github.com/monopole/hello
commonLabels:
  app: my-hello

resources:
- deployment.yaml
- service.yaml
- configMap.yaml`
	f.WriteFile("kustomization.yaml", kustomizeFile)

	configMap := `apiVersion: v1
kind: ConfigMap
metadata:
  name: the-map
data:
  altGreeting: "Good Morning!"
  enableRisky: "false"`
	f.WriteFile("configMap.yaml", configMap)

	deployment := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: the-deployment
spec:
  replicas: 3
  template:
    metadata:
      labels:
        deployment: hello
    spec:
      containers:
      - name: the-container
        image: monopole/hello:1
        command: ["/hello",
                  "--port=8080",
                  "--enableRiskyFeature=$(ENABLE_RISKY)"]
        ports:
        - containerPort: 8080
        env:
        - name: ALT_GREETING
          valueFrom:
            configMapKeyRef:
              name: the-map
              key: altGreeting
        - name: ENABLE_RISKY
          valueFrom:
            configMapKeyRef:
              name: the-map
              key: enableRisky`

	f.WriteFile("deployment.yaml", deployment)

	service := `kind: Service
apiVersion: v1
metadata:
  name: the-service
spec:
  selector:
    deployment: hello
  type: LoadBalancer
  ports:
  - protocol: TCP
    port: 8666
    targetPort: 8080`

	f.WriteFile("service.yaml", service)

	f.WriteFile("Dockerfile", "dockerfile text")
	f.WriteFile("Tiltfile", `
def blorgly():
  yaml = kustomize(".")
  s = k8s_service(yaml, static_build("Dockerfile", "docker-tag"))
  return s
`)

	manifest := f.LoadManifest("blorgly")
	expected := f.JoinPaths([]string{
		"Dockerfile",
		"Tiltfile",
		"configMap.yaml",
		"deployment.yaml",
		"kustomization.yaml",
		"service.yaml",
	})
	assert.Equal(t, expected, manifest.ConfigFiles)
}

func TestGlobalYAML(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("global.yaml", "this is the global yaml")
	f.WriteFile("Tiltfile", `yaml = read_file('./global.yaml')
global_yaml(yaml)`)

	tiltconfig, err := Load(f.ctx, FileName)
	if err != nil {
		f.T().Fatal("loading tiltconfig:", err)
	}
	assert.Equal(t, tiltconfig.globalYAMLStr, "this is the global yaml")
	assert.Equal(t, tiltconfig.globalYAMLDeps, []string{f.JoinPath("global.yaml")})
}

func TestGlobalYAMLMultipleCallsThrowsError(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `global_yaml('abc')
global_yaml('def')`)

	_, err := Load(f.ctx, FileName)
	if assert.Error(t, err, "expect multiple invocations of `global_yaml` to result in error") {
		assert.Equal(t, err.Error(), "`global_yaml` can be called only once per Tiltfile")
	}
}

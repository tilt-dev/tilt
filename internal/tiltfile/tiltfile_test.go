package tiltfile

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestGetServiceConfig(t *testing.T) {
	dockerfile := tempFile("docker text")
	file := tempFile(
		fmt.Sprintf(`def blorgly():
  image = build_docker_image("%v", "docker tag", "the entrypoint")
  image.add('/mount_points/1', local_git_repo('.'))
  print(image.file_name)
  image.run("go install github.com/windmilleng/blorgly-frontend/server/...")
  image.run("echo hi")
  return k8s_service("yaaaaaaaaml", image)
`, dockerfile))
	defer os.Remove(file)
	defer os.Remove(dockerfile)

	tiltconfig, err := Load(file)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	serviceConfig, err := tiltconfig.GetServiceConfigs("blorgly")
	if err != nil {
		t.Fatal("getting service config:", err)
	}

	service := serviceConfig[0]
	assert.Equal(t, "docker text", service.DockerfileText)
	assert.Equal(t, "docker tag", service.DockerfileTag)
	assert.Equal(t, "yaaaaaaaaml", service.K8sYaml)
	assert.Equal(t, 1, len(service.Mounts))
	assert.Equal(t, "/mount_points/1", service.Mounts[0].ContainerPath)
	assert.Equal(t, ".", service.Mounts[0].Repo.LocalPath)
	assert.Equal(t, 2, len(service.Steps))
	assert.Equal(t, []string{"sh", "-c", "go install github.com/windmilleng/blorgly-frontend/server/..."}, service.Steps[0].Argv)
	assert.Equal(t, []string{"sh", "-c", "echo hi"}, service.Steps[1].Argv)
	assert.Equal(t, []string{"sh", "-c", "the entrypoint"}, service.Entrypoint.Argv)
}

func TestGetServiceConfigMissingDockerFile(t *testing.T) {
	file := tempFile(
		fmt.Sprintf(`def blorgly():
  image = build_docker_image("asfaergiuhaeriguhaergiu", "docker tag", "the entrypoint")
  print(image.file_name)
  image.run("go install github.com/windmilleng/blorgly-frontend/server/...")
  image.run("echo hi")
  return k8s_service("yaaaaaaaaml", image)
`))
	defer os.Remove(file)

	tiltconfig, err := Load(file)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	_, err = tiltconfig.GetServiceConfigs("blorgly")
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

	tiltConfig, err := Load(file)
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
  return composite_service({"blorgly_backend": blorgly_backend(), "blorgly_frontend": blorgly_frontend()})

def blorgly_backend():
    image = build_docker_image("%v", "docker tag", "the entrypoint")
    print(image.file_name)
    image.run("go install github.com/windmilleng/blorgly-frontend/server/...")
    image.run("echo hi")
    return k8s_service("yaml", image)

def blorgly_frontend():
  image = build_docker_image("%v", "docker tag", "the entrypoint")
  print(image.file_name)
  image.run("go install github.com/windmilleng/blorgly-frontend/server/...")
  image.run("echo hi")
  return k8s_service("yaaaaaaaaml", image)
`, dockerfile, dockerfile))
	defer os.Remove(file)
	defer os.Remove(dockerfile)

	tiltConfig, err := Load(file)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	serviceConfig, err := tiltConfig.GetServiceConfigs("blorgly")
	if err != nil {
		t.Fatal("getting service config:", err)
	}

	assert.Equal(t, "blorgly_backend", serviceConfig[0].Name.String())
	assert.Equal(t, "blorgly_frontend", serviceConfig[1].Name.String())
}

func TestGetServiceConfigUndefined(t *testing.T) {
	file := tempFile(
		`def blorgly():
  return "yaaaaaaaml"
`)
	defer os.Remove(file)

	tiltConfig, err := Load(file)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	_, err = tiltConfig.GetServiceConfigs("blorgly2")
	if err == nil {
		t.Fatal("expected error b/c of undefined service config:")
	}

	for _, s := range []string{"does not define", "blorgly2"} {
		assert.True(t, strings.Contains(err.Error(), s))
	}
}

func TestGetServiceConfigNonFunction(t *testing.T) {
	file := tempFile("blorgly2 = 3")
	defer os.Remove(file)

	tiltConfig, err := Load(file)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	_, err = tiltConfig.GetServiceConfigs("blorgly2")
	if assert.NotNil(t, err, "GetServiceConfigs did not return an error") {
		for _, s := range []string{"blorgly2", "function", "int"} {
			assert.True(t, strings.Contains(err.Error(), s))
		}
	}
}

func TestGetServiceConfigTakesArgs(t *testing.T) {
	file := tempFile(
		`def blorgly2(x):
			return "foo"
`)
	defer os.Remove(file)

	tiltConfig, err := Load(file)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	_, err = tiltConfig.GetServiceConfigs("blorgly2")
	if assert.NotNil(t, err, "GetServiceConfigs did not return an error") {
		for _, s := range []string{"blorgly2", "0 arguments"} {
			assert.True(t, strings.Contains(err.Error(), s))
		}
	}
}

func TestGetServiceConfigRaisesError(t *testing.T) {
	file := tempFile(
		`def blorgly2():
			"foo"[10]`) // index out of range
	defer os.Remove(file)

	tiltConfig, err := Load(file)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	_, err = tiltConfig.GetServiceConfigs("blorgly2")
	if assert.NotNil(t, err, "GetServiceConfigs did not return an error") {
		for _, s := range []string{"blorgly2", "string index", "out of range"} {
			assert.True(t, strings.Contains(err.Error(), s), "error message '%V' did not contain '%V'", err.Error(), s)
		}
	}
}

func TestGetServiceConfigReturnsWrongType(t *testing.T) {
	file := tempFile(
		`def blorgly2():
			return "foo"`) // index out of range
	defer os.Remove(file)

	tiltConfig, err := Load(file)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	_, err = tiltConfig.GetServiceConfigs("blorgly2")
	if assert.NotNil(t, err, "GetServiceConfigs did not return an error") {
		for _, s := range []string{"blorgly2", "string", "k8s_service"} {
			assert.True(t, strings.Contains(err.Error(), s), "error message '%V' did not contain '%V'", err.Error(), s)
		}
	}
}

func TestGetServiceConfigLocalReturnsNon0(t *testing.T) {
	file := tempFile(
		`def blorgly2():
			local('echo "foo" "bar" && echo "baz" "quu" >&2 && exit 1')`) // index out of range
	defer os.Remove(file)

	tiltConfig, err := Load(file)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	_, err = tiltConfig.GetServiceConfigs("blorgly2")
	if assert.NotNil(t, err, "GetServiceConfigs did not return an error") {
		// "foo bar" and "baz quu" are separated above so that the match below only matches the strings in the output,
		// not in the command
		for _, s := range []string{"blorgly2", "exit status 1", "foo bar", "baz quu"} {
			assert.True(t, strings.Contains(err.Error(), s), "error message '%v' did not contain '%v'", err.Error(), s)
		}
	}
}

func TestGetServiceConfigWithLocalCmd(t *testing.T) {
	dockerfile := tempFile("docker text")
	file := tempFile(
		fmt.Sprintf(`def blorgly():
  image = build_docker_image("%v", "docker tag", "the entrypoint")
  print(image.file_name)
  image.run("go install github.com/windmilleng/blorgly-frontend/server/...")
  image.run("echo hi")
  yaml = local('echo yaaaaaaaaml')
  return k8s_service(yaml, image)
`, dockerfile))
	defer os.Remove(file)
	defer os.Remove(dockerfile)

	tiltconfig, err := Load(file)
	if err != nil {
		t.Fatal("loading tiltconfig:", err)
	}

	serviceConfig, err := tiltconfig.GetServiceConfigs("blorgly")
	if err != nil {
		t.Fatal("getting service config:", err)
	}

	service := serviceConfig[0]
	assert.Equal(t, "docker text", service.DockerfileText)
	assert.Equal(t, "docker tag", service.DockerfileTag)
	assert.Equal(t, "yaaaaaaaaml\n", service.K8sYaml)
	assert.Equal(t, 2, len(service.Steps))
	assert.Equal(t, []string{"sh", "-c", "go install github.com/windmilleng/blorgly-frontend/server/..."}, service.Steps[0].Argv)
	assert.Equal(t, []string{"sh", "-c", "echo hi"}, service.Steps[1].Argv)
	assert.Equal(t, []string{"sh", "-c", "the entrypoint"}, service.Entrypoint.Argv)
}

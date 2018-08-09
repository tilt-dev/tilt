package tiltfile

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"
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
  image = build_docker_image("%v", "docker tag")
  print(image.file_name)
  image.add_cmd("go install github.com/windmilleng/blorgly-frontend/server/...")
  image.add_cmd("echo hi")
  return k8s_service("yaaaaaaaaml", image)
`, dockerfile))
	defer os.Remove(file)
	defer os.Remove(dockerfile)
	tiltconfig, err := Load(file)
	assert.Nil(t, err)
	serviceConfig, err := tiltconfig.GetServiceConfig("blorgly")
	assert.Nil(t, err)
	assert.Equal(t, "docker text", serviceConfig.DockerfileText)
	assert.Equal(t, "docker tag", serviceConfig.DockerfileTag)
	assert.Equal(t, "yaaaaaaaaml", serviceConfig.K8SYaml)
	assert.Equal(t, 2, len(serviceConfig.Steps))
	assert.Equal(t, []string{"bash", "-c", "go install github.com/windmilleng/blorgly-frontend/server/..."}, serviceConfig.Steps[0].Argv)
	assert.Equal(t, []string{"bash", "-c", "echo hi"}, serviceConfig.Steps[1].Argv)
}

func TestGetServiceConfigMissingDockerFile(t *testing.T) {
	file := tempFile(
		fmt.Sprintf(`def blorgly():
  image = build_docker_image("asfaergiuhaeriguhaergiu", "docker tag")
  print(image.file_name)
  image.add_cmd("go install github.com/windmilleng/blorgly-frontend/server/...")
  image.add_cmd("echo hi")
  return k8s_service("yaaaaaaaaml", image)
`))
	defer os.Remove(file)
	tiltconfig, err := Load(file)
	assert.Nil(t, err)
	_, err = tiltconfig.GetServiceConfig("blorgly")
	assert.NotNil(t, err)
	for _, s := range []string{"asfaergiuhaeriguhaergiu", "no such file or directory"} {
		assert.True(t, strings.Contains(err.Error(), s))
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
	for _, s := range []string{"blorgly", "blorgly_backend", "blorgly_frontend"} {
		assert.Contains(t, tiltConfig.globals, s)
	}
	assert.Nil(t, err)
}

func TestGetServiceConfigUndefined(t *testing.T) {
	file := tempFile(
		`def blorgly():
  return "yaaaaaaaml"
`)
	defer os.Remove(file)
	tiltConfig, err := Load(file)
	assert.Nil(t, err)
	_, err = tiltConfig.GetServiceConfig("blorgly2")
	for _, s := range []string{"does not define", "blorgly2"} {
		assert.True(t, strings.Contains(err.Error(), s))
	}
}

func TestGetServiceConfigNonFunction(t *testing.T) {
	file := tempFile("blorgly2 = 3")
	defer os.Remove(file)
	tiltConfig, err := Load(file)
	assert.Nil(t, err)
	_, err = tiltConfig.GetServiceConfig("blorgly2")
	for _, s := range []string{"blorgly2", "function", "int"} {
		assert.True(t, strings.Contains(err.Error(), s))
	}
}

func TestGetServiceConfigTakesArgs(t *testing.T) {
	file := tempFile(
		`def blorgly2(x):
			return "foo"
`)
	defer os.Remove(file)
	tiltConfig, err := Load(file)
	assert.Nil(t, err)
	_, err = tiltConfig.GetServiceConfig("blorgly2")
	for _, s := range []string{"blorgly2", "0 arguments"} {
		assert.True(t, strings.Contains(err.Error(), s))
	}
}

func TestGetServiceConfigRaisesError(t *testing.T) {
	file := tempFile(
		`def blorgly2():
			"foo"[10]`) // index out of range
	defer os.Remove(file)
	tiltConfig, err := Load(file)
	_, err = tiltConfig.GetServiceConfig("blorgly2")
	assert.NotNil(t, err, "GetServiceConfig did not return an error")
	for _, s := range []string{"blorgly2", "string index", "out of range"} {
		assert.True(t, strings.Contains(err.Error(), s), "error message '%V' did not contain '%V'", err.Error(), s)
	}
}

func TestGetServiceConfigReturnsWrongType(t *testing.T) {
	file := tempFile(
		`def blorgly2():
			return "foo"`) // index out of range
	defer os.Remove(file)
	tiltConfig, err := Load(file)
	_, err = tiltConfig.GetServiceConfig("blorgly2")
	assert.NotNil(t, err, "GetServiceConfig did not return an error")
	for _, s := range []string{"blorgly2", "string", "k8s_service"} {
		assert.True(t, strings.Contains(err.Error(), s), "error message '%V' did not contain '%V'", err.Error(), s)
	}
}

func TestGetServiceConfigLocalReturnsNon0(t *testing.T) {
	file := tempFile(
		`def blorgly2():
			local('echo "foo" "bar" && echo "baz" "quu" >&2 && exit 1')`) // index out of range
	defer os.Remove(file)
	tiltConfig, err := Load(file)
	assert.Nil(t, err)
	_, err = tiltConfig.GetServiceConfig("blorgly2")
	// "foo bar" and "baz quu" are separated above so that the match below only matches the strings in the output,
	// not in the command
	for _, s := range []string{"blorgly2", "exit status 1", "foo bar", "baz quu"} {
		assert.True(t, strings.Contains(err.Error(), s), "error message '%V' did not contain '%V'", err.Error(), s)
	}
}

func TestGetServiceConfigWithLocalCmd(t *testing.T) {
	dockerfile := tempFile("docker text")
	file := tempFile(
		fmt.Sprintf(`def blorgly():
  image = build_docker_image("%v", "docker tag")
  print(image.file_name)
  image.add_cmd("go install github.com/windmilleng/blorgly-frontend/server/...")
  image.add_cmd("echo hi")
  yaml = local('echo yaaaaaaaaml')
  return k8s_service(yaml, image)
`, dockerfile))
	defer os.Remove(file)
	defer os.Remove(dockerfile)
	tiltconfig, err := Load(file)
	assert.Nil(t, err)
	serviceConfig, err := tiltconfig.GetServiceConfig("blorgly")
	assert.Nil(t, err)
	assert.Equal(t, "docker text", serviceConfig.DockerfileText)
	assert.Equal(t, "docker tag", serviceConfig.DockerfileTag)
	assert.Equal(t, "yaaaaaaaaml\n", serviceConfig.K8SYaml)
	assert.Equal(t, 2, len(serviceConfig.Steps))
	assert.Equal(t, []string{"bash", "-c", "go install github.com/windmilleng/blorgly-frontend/server/..."}, serviceConfig.Steps[0].Argv)
	assert.Equal(t, []string{"bash", "-c", "echo hi"}, serviceConfig.Steps[1].Argv)
}
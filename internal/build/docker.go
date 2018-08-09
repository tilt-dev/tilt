package build

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type Mount struct {
	// TODO(dmiller) make this more generic
	Repo          LocalGithubRepo
	ContainerPath string
}

type Repo interface{}

type LocalGithubRepo struct {
	LocalPath string
}

type Cmd struct {
	argv []string
}

type DockerBuilder interface {
	BuildDocker(ctx context.Context, baseDockerfile string, mounts []Mount, cmds []Cmd, tag string) (string, error)
}

type localDockerBuilder struct {
	dcli *client.Client
}

// NOTE(dmiller): not fully implemented yet
func (l *localDockerBuilder) BuildDocker(ctx context.Context, baseDockerfile string, mounts []Mount, cmds []Cmd, tag string) (string, error) {
	baseTag, err := l.buildBase(ctx, baseDockerfile, tag)
	if err != nil {
		return "", err
	}

	// TODO(dmiller): mounts
	// TODO(dmiller): steps

	return baseTag, nil
}

func (l *localDockerBuilder) buildBase(ctx context.Context, baseDockerfile string, tag string) (string, error) {
	tar, err := tarFromDockerfile(baseDockerfile)
	if err != nil {
		return "", err
	}
	imageBuildResponse, err := l.dcli.ImageBuild(
		ctx,
		tar,
		types.ImageBuildOptions{
			Context:    tar,
			Dockerfile: "Dockerfile",
			Tags:       []string{tag},
		})

	defer imageBuildResponse.Body.Close()
	output := &strings.Builder{}
	_, err = io.Copy(output, imageBuildResponse.Body)
	if err != nil {
		return "", err
	}

	return getDigestFromOutput(output.String())
}

func tarFromDockerfile(df string) (*bytes.Reader, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer tw.Close()

	tarHeader := &tar.Header{
		Name: "Dockerfile",
		Size: int64(len(df)),
	}
	err := tw.WriteHeader(tarHeader)
	if err != nil {
		return nil, err
	}
	_, err = tw.Write([]byte(df))
	if err != nil {
		return nil, err
	}
	dockerFileTarReader := bytes.NewReader(buf.Bytes())

	return dockerFileTarReader, nil
}

func getDigestFromOutput(output string) (string, error) {
	re := regexp.MustCompile("Successfully built ([[:alnum:]]*)")
	res := re.FindStringSubmatch(output)
	if len(res) != 2 {
		return "", fmt.Errorf("Expected to get two matches for regex, but for %d", len(res))
	}
	return res[1], nil
}

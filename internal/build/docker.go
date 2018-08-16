package build

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/windmilleng/tilt/internal/model"

	"github.com/docker/cli/cli/command"
	cliflags "github.com/docker/cli/cli/flags"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/registry"
	"github.com/opencontainers/go-digest"
)

// Tilt always pushes to the same tag. We never push to latest.
const pushTag = "wm-tilt"

var ErrEntrypointInDockerfile = errors.New("base Dockerfile contains an ENTRYPOINT, " +
	"which is not currently supported -- provide an entrypoint in your Tiltfile")

type localDockerBuilder struct {
	dcli *client.Client
}

type Builder interface {
	BuildDocker(ctx context.Context, baseDockerfile string, mounts []model.Mount, steps []model.Cmd, entrypoint model.Cmd) (digest.Digest, error)
	BuildDockerFromExisting(ctx context.Context, existing digest.Digest, mounts []model.Mount, steps []model.Cmd, entrypoint model.Cmd) (digest.Digest, error)
	PushDocker(ctx context.Context, name reference.Named, dig digest.Digest) (digest.Digest, error)
}

type pushOutput struct {
	Tag    string
	Digest string
	Size   int
}

var _ Builder = &localDockerBuilder{}

func NewLocalDockerBuilder(cli *client.Client) *localDockerBuilder {
	return &localDockerBuilder{cli}
}

func (l *localDockerBuilder) BuildDocker(ctx context.Context, baseDockerfile string,
	mounts []model.Mount, steps []model.Cmd, entrypoint model.Cmd) (digest.Digest, error) {
	baseDigest, err := l.buildBaseWithMounts(ctx, baseDockerfile, mounts)
	if err != nil {
		return "", fmt.Errorf("buildBaseWithMounts: %v", err)
	}

	newDigest, err := l.execStepsOnImage(ctx, baseDigest, steps)
	if err != nil {
		return "", fmt.Errorf("execStepsOnImage: %v", err)
	}

	newDigest, err = l.imageWithEntrypoint(ctx, newDigest, entrypoint)
	if err != nil {
		return "", fmt.Errorf("imageWithEntrypoint: %v", err)
	}

	return newDigest, nil
}

func (l *localDockerBuilder) BuildDockerFromExisting(ctx context.Context, existing digest.Digest,
	mounts []model.Mount, steps []model.Cmd, entrypoint model.Cmd) (digest.Digest, error) {
	// TODO(maia): unset entrypoint from existing so that we can exec steps on it
	// (Maybe don't pass entrypoint, and instead read and parse existing entrypoint and reapply?)
	dfForExisting := fmt.Sprintf("FROM %s", existing.Encoded())
	return l.BuildDocker(ctx, dfForExisting, mounts, steps, entrypoint)
}

// Naively tag the digest and push it up to the docker registry specified in the name.
//
// TODO(nick) In the future, I would like us to be smarter about checking if the kubernetes cluster
// we're running in has access to the given registry. And if it doesn't, we should either emit an
// error, or push to a registry that kubernetes does have access to (e.g., a local registry).
func (l *localDockerBuilder) PushDocker(ctx context.Context, ref reference.Named, dig digest.Digest) (digest.Digest, error) {
	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return "", fmt.Errorf("PushDocker#ParseRepositoryInfo: %s", err)
	}

	cli := command.NewDockerCli(nil, os.Stdout, os.Stderr, true)
	err = cli.Initialize(cliflags.NewClientOptions())
	if err != nil {
		return "", fmt.Errorf("PushDocker#InitializeCLI: %s", err)
	}
	authConfig := command.ResolveAuthConfig(ctx, cli, repoInfo.Index)
	requestPrivilege := command.RegistryAuthenticationPrivilegedFunc(cli, repoInfo.Index, "push")

	encodedAuth, err := command.EncodeAuthToBase64(authConfig)
	if err != nil {
		return "", fmt.Errorf("PushDocker#EncodeAuthToBase64: %s", err)
	}

	options := types.ImagePushOptions{
		RegistryAuth:  encodedAuth,
		PrivilegeFunc: requestPrivilege,
	}

	if reference.Domain(ref) == "" {
		return "", fmt.Errorf("PushDocker: no domain in container name: %s", ref)
	}

	tag, err := reference.WithTag(ref, pushTag)
	if err != nil {
		return "", fmt.Errorf("PushDocker: %v", err)
	}

	err = l.dcli.ImageTag(ctx, dig.String(), tag.String())
	if err != nil {
		return "", fmt.Errorf("PushDocker#ImageTag: %v", err)
	}

	imagePushResponse, err := l.dcli.ImagePush(
		ctx,
		tag.String(),
		options)
	if err != nil {
		return "", fmt.Errorf("PushDocker#ImagePush: %v", err)
	}

	defer imagePushResponse.Close()
	pushedDigest, err := getDigestFromPushOutput(imagePushResponse)
	if err != nil {
		return "", fmt.Errorf("PushDocker#getDigestFromPushOutput: %v", err)
	}

	return pushedDigest, nil
}

func (l *localDockerBuilder) buildBaseWithMounts(ctx context.Context, baseDockerfile string, mounts []model.Mount) (digest.Digest, error) {
	err := validateDockerfile(baseDockerfile)
	if err != nil {
		return "", err
	}

	archive, err := tarContext(baseDockerfile, mounts)
	if err != nil {
		return "", err
	}
	// TODO(dmiller): remove this debugging code
	//tar2, err := tarContext(baseDockerfile, mounts)
	//if err != nil {
	//	return "", err
	//}
	//buf := new(bytes.Buffer)
	//buf.ReadFrom(tar2)
	//err = ioutil.WriteFile("/tmp/debug.tar", buf.Bytes(), os.FileMode(0777))
	//if err != nil {
	//	return "", err
	//}

	imageBuildResponse, err := l.dcli.ImageBuild(
		ctx,
		archive,
		types.ImageBuildOptions{
			Context:    archive,
			Dockerfile: "Dockerfile",
			Remove:     shouldRemoveImage(),
		})
	if err != nil {
		return "", err
	}

	defer imageBuildResponse.Body.Close()
	result, err := readDockerOutput(imageBuildResponse.Body)
	if err != nil {
		return "", fmt.Errorf("ImageBuild: %v", err)
	}

	return getDigestFromAux(*result)
}

// NOTE(maia): can put more logic in here sometime; currently just returns an error
// if Dockerfile contains an ENTRYPOINT, which is illegal in Tilt right now (an
// ENTRYPOINT overrides a ContainerCreate Cmd, which we rely on).
// TODO: extract the ENTRYPOINT line from the Dockerfile and reapply it later.
func validateDockerfile(df string) error {
	for _, line := range strings.Split(df, "\n") {
		if strings.HasPrefix(line, "ENTRYPOINT") {
			return ErrEntrypointInDockerfile
		}
	}
	return nil
}

func (l *localDockerBuilder) execStepsOnImage(ctx context.Context, img digest.Digest, steps []model.Cmd) (digest.Digest, error) {
	imageWithSteps := img
	for _, s := range steps {
		cId, err := l.startContainer(ctx, containerConfigRunCmd(imageWithSteps, s))
		if err != nil {
			return "", fmt.Errorf("startContainer '%s': %v", img, err)
		}

		id, err := l.dcli.ContainerCommit(ctx, cId, types.ContainerCommitOptions{})
		if err != nil {
			return "", fmt.Errorf("containerCommit: %v", err)
		}
		imageWithSteps = digest.Digest(id.ID)
	}

	return digest.Digest(imageWithSteps), nil
}

// TODO(maia): can probs do this in a more efficient place -- e.g. `execStepsOnImage`
// already spins up + commits a container, maybe piggyback off that?
func (l *localDockerBuilder) imageWithEntrypoint(ctx context.Context, img digest.Digest, entrypoint model.Cmd) (digest.Digest, error) {
	cId, err := l.startContainer(ctx, containerConfigRunCmd(img, model.Cmd{}))
	if err != nil {
		return "", fmt.Errorf("startContainer '%s': %v", string(img), err)
	}

	// Only attach the entrypoint if there's something in it
	opts := types.ContainerCommitOptions{}
	if !entrypoint.Empty() {
		opts.Changes = []string{entrypoint.EntrypointStr()}
	}

	id, err := l.dcli.ContainerCommit(ctx, cId, opts)
	if err != nil {
		return "", fmt.Errorf("containerCommit: %v", err)
	}

	return digest.Digest(id.ID), nil
}

// startContainer starts a container from the given config.
// Returns the container id iff the container successfully runs; else, error.
func (l *localDockerBuilder) startContainer(ctx context.Context, config *container.Config) (cId string, err error) {
	resp, err := l.dcli.ContainerCreate(ctx, config, nil, nil, "")
	if err != nil {
		return "", err
	}
	containerID := resp.ID

	err = l.dcli.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
	if err != nil {
		return "", err
	}

	statusCh, errCh := l.dcli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return "", err
		}
	case status := <-statusCh:
		if status.Error != nil {
			return "", errors.New(status.Error.Message)
		}
		// TODO(matt) feed this reader into the logger
		r, err := l.dcli.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
		if err != nil {
			return "", err
		}
		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(r)
		if err != nil {
			return "", err
		}
		if status.StatusCode != 0 {
			return "", fmt.Errorf("container '%+v' had non-0 exit code %v. output: '%v'", config, status.StatusCode, string(buf.Bytes()))
		}
	}

	return containerID, nil
}

// tarContext amends the dockerfile with appropriate ADD statements,
// and returns that new dockerfile + necessary files in a tar
func tarContext(df string, mounts []model.Mount) (*bytes.Reader, error) {
	buf := new(bytes.Buffer)

	// We'll tar all mounts so that their contents live inside their own dest
	// directories; since we generate the tar properly, can just dump everything
	// from the root into the container at /
	newdf := fmt.Sprintf("%s\nADD . /", df)

	err := tarToBuf(buf, newdf, mounts)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(buf.Bytes()), nil
}

func tarToBuf(buf *bytes.Buffer, df string, mounts []model.Mount) error {
	tw := tar.NewWriter(buf)
	defer func() {
		err := tw.Close()
		if err != nil {
			log.Printf("Error closing tar writer: %s", err.Error())
		}
	}()

	tarHeader := &tar.Header{
		Name: "Dockerfile",
		Size: int64(len(df)),
	}
	err := tw.WriteHeader(tarHeader)
	if err != nil {
		return err
	}
	_, err = tw.Write([]byte(df))
	if err != nil {
		return err
	}

	for _, m := range mounts {
		err = tarPath(tw, m.Repo.LocalPath, m.ContainerPath)
		if err != nil {
			return err
		}
	}
	return nil
}

// tarPath writes the given source path into tarWriter at the given dest (recursively for directories).
// e.g. tarring my_dir --> dest d: d/file_a, d/file_b
func tarPath(tarWriter *tar.Writer, source, dest string) error {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("%s: stat: %v", source, err)
	}

	sourceIsDir := sourceInfo.IsDir()
	if sourceIsDir {
		// Make sure we can trim this off filenames to get valid relative filepaths
		if !strings.HasSuffix(source, "/") {
			source += "/"
		}
	}

	dest = strings.TrimPrefix(dest, "/")

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking to %s: %v", path, err)
		}

		header, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return fmt.Errorf("%s: making header: %v", path, err)
		}

		if sourceIsDir {
			// Name of file in tar should be relative to source directory...
			header.Name = strings.TrimPrefix(path, source)
		}

		if dest != "" {
			// ...and live inside `dest` (if given)
			header.Name = filepath.Join(dest, header.Name)
		}

		err = tarWriter.WriteHeader(header)
		if err != nil {
			return fmt.Errorf("%s: writing header: %v", path, err)
		}

		if info.IsDir() {
			return nil
		}

		if header.Typeflag == tar.TypeReg {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("%s: open: %v", path, err)
			}
			defer file.Close()

			_, err = io.CopyN(tarWriter, file, info.Size())
			if err != nil && err != io.EOF {
				return fmt.Errorf("%s: copying contents: %v", path, err)
			}
		}
		return nil
	})
}

// Docker API commands stream back a sequence of JSON messages.
//
// The result of the command is in a JSON object with field "aux".
//
// Errors are reported in a JSON object with field "errorDetail"
//
// NOTE(nick): I haven't found a good document describing this protocol
// but you can find it implemented in Docker here:
// https://github.com/moby/moby/blob/1da7d2eebf0a7a60ce585f89a05cebf7f631019c/pkg/jsonmessage/jsonmessage.go#L139
func readDockerOutput(reader io.Reader) (*json.RawMessage, error) {
	var result *json.RawMessage
	decoder := json.NewDecoder(reader)
	for decoder.More() {
		message := jsonmessage.JSONMessage{}
		err := decoder.Decode(&message)
		if err != nil {
			return nil, fmt.Errorf("decoding docker output: %v", err)
		}

		if message.ErrorMessage != "" {
			return nil, errors.New(message.ErrorMessage)
		}

		if message.Error != nil {
			return nil, errors.New(message.Error.Message)
		}

		if message.Aux != nil {
			result = message.Aux
		}
	}
	return result, nil
}

func getDigestFromBuildOutput(reader io.Reader) (digest.Digest, error) {
	aux, err := readDockerOutput(reader)
	if err != nil {
		return "", err
	}
	if aux == nil {
		return "", fmt.Errorf("getDigestFromBuildOutput: No results found in docker output")
	}
	return getDigestFromAux(*aux)
}

func getDigestFromPushOutput(reader io.Reader) (digest.Digest, error) {
	aux, err := readDockerOutput(reader)
	if err != nil {
		return "", err
	}
	d := pushOutput{}
	err = json.Unmarshal(*aux, &d)
	if err != nil {
		return "", fmt.Errorf("getDigestFromPushOutput#Unmarshal: %v, json string: %+v", err, aux)
	}

	if d.Digest == "" {
		return "", fmt.Errorf("getDigestFromPushOutput: Digest not found in %+v", aux)
	}

	return digest.Digest(d.Digest), nil
}

func getDigestFromAux(aux json.RawMessage) (digest.Digest, error) {
	digestMap := make(map[string]string, 0)
	err := json.Unmarshal(aux, &digestMap)
	if err != nil {
		return "", fmt.Errorf("getDigestFromAux: %v", err)
	}

	id, ok := digestMap["ID"]
	if !ok {
		return "", fmt.Errorf("getDigestFromAux: ID not found")
	}
	return digest.Digest(id), nil
}

func shouldRemoveImage() bool {
	if flag.Lookup("test.v") == nil {
		return false
	}
	return true
}

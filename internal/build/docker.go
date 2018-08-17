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
	BuildDockerFromScratch(ctx context.Context, baseDockerfile string, mounts []model.Mount, steps []model.Cmd, entrypoint model.Cmd) (digest.Digest, error)
	BuildDockerFromExisting(ctx context.Context, existing digest.Digest, paths []PathMapping, steps []model.Cmd) (digest.Digest, error)
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

func (l *localDockerBuilder) BuildDockerFromScratch(ctx context.Context, baseDockerfile string,
	mounts []model.Mount, steps []model.Cmd, entrypoint model.Cmd) (digest.Digest, error) {
	return l.buildDocker(ctx, baseDockerfile, MountsToPaths(mounts), steps, entrypoint)
}

func (l *localDockerBuilder) BuildDockerFromExisting(ctx context.Context, existing digest.Digest,
	paths []PathMapping, steps []model.Cmd) (digest.Digest, error) {
	dfForExisting := fmt.Sprintf("FROM %s", existing.Encoded())
	return l.buildDocker(ctx, dfForExisting, paths, steps, model.Cmd{})
}

func (l *localDockerBuilder) buildDocker(ctx context.Context, df string,
	paths []PathMapping, steps []model.Cmd, entrypoint model.Cmd) (digest.Digest, error) {
	baseDigest, err := l.buildFromDfWithFiles(ctx, df, paths)
	if err != nil {
		return "", fmt.Errorf("buildFromDfWithFiles: %v", err)
	}

	newDigest, err := l.execStepsOnImage(ctx, baseDigest, steps)
	if err != nil {
		return "", fmt.Errorf("execStepsOnImage: %v", err)
	}

	if !entrypoint.Empty() {
		newDigest, err = l.imageWithEntrypoint(ctx, newDigest, entrypoint)
		if err != nil {
			return "", fmt.Errorf("imageWithEntrypoint: %v", err)
		}
	}

	return newDigest, nil
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

func (l *localDockerBuilder) buildFromDfWithFiles(ctx context.Context, df string, paths []PathMapping) (digest.Digest, error) {
	err := validateDockerfile(df)
	if err != nil {
		return "", err
	}

	archive, err := TarContext(df, paths)
	if err != nil {
		return "", err
	}
	// TODO(dmiller): remove this debugging code
	//tar2, err := TarContext(df, mounts)
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

func TarContext(df string, paths []PathMapping) (*bytes.Reader, error) {
	buf, err := tarContext(df, paths)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(buf.Bytes()), nil
}

// tarContext amends the dockerfile with appropriate ADD statements,
// and returns that new dockerfile + necessary files in a tar
// NOTE(maia) now that there's more stuff to this, maybe want a more illustrative name for this?
// TODO(maia): do we need BOTH paths and mounts??
func tarContext(df string, paths []PathMapping) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer func() {
		err := tw.Close()
		if err != nil {
			log.Printf("Error closing tar writer: %s", err.Error())
		}
	}()

	// TODO: maybe write our own tarWriter struct with methods on it, so it's clearer that we're modifying the tar writer in place
	dnePaths, err := archiveIfExists(tw, paths)
	if err != nil {
		return nil, fmt.Errorf("archiveIfExists: %v", err)
	}

	newDf := updateDf(df, dnePaths)
	err = archiveDf(tw, newDf)
	if err != nil {
		return nil, fmt.Errorf("archiveDf: %v", err)
	}

	return buf, nil
}

func updateDf(df string, dnePaths []PathMapping) string {
	// Add 'ADD' statements (right now just add whatever's in context;
	// this is safe b/c only adds/overwrites, doesn't remove).
	newDf := fmt.Sprintf("%s\nADD . /", df)

	if len(dnePaths) > 0 {
		// Add 'rm' statements; if changed file was deleted locally, remove if from container
		rmCmd := strings.Builder{}
		rmCmd.WriteString("rm") // sh -c?
		for _, p := range dnePaths {
			rmCmd.WriteString(fmt.Sprintf(" %s", p.ContainerPath))
		}
		newDf = fmt.Sprintf("%s\nRUN %s", newDf, rmCmd.String())
	}
	return newDf
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

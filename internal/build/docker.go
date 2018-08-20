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

	"github.com/windmilleng/tilt/internal/logger"
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
	pool containerPool
}

type Builder interface {
	BuildDockerFromScratch(ctx context.Context, baseDockerfile Dockerfile, mounts []model.Mount, steps []model.Cmd, entrypoint model.Cmd) (digest.Digest, error)
	BuildDockerFromExisting(ctx context.Context, existing digest.Digest, paths []pathMapping, steps []model.Cmd) (digest.Digest, error)
	PushDocker(ctx context.Context, name reference.Named, dig digest.Digest) (digest.Digest, error)
}

type pushOutput struct {
	Tag    string
	Digest string
	Size   int
}

var _ Builder = &localDockerBuilder{}

func NewLocalDockerBuilder(dcli *client.Client) *localDockerBuilder {
	return &localDockerBuilder{dcli: dcli, pool: newContainerPool(dcli)}
}

func (l *localDockerBuilder) BuildDockerFromScratch(ctx context.Context, baseDockerfile Dockerfile,
	mounts []model.Mount, steps []model.Cmd, entrypoint model.Cmd) (digest.Digest, error) {
	return l.buildDocker(ctx, baseDockerfile, MountsToPath(mounts), steps, entrypoint)
}

func (l *localDockerBuilder) BuildDockerFromExisting(ctx context.Context, existing digest.Digest,
	paths []pathMapping, steps []model.Cmd) (digest.Digest, error) {
	dfForExisting := DockerfileFromExisting(existing)
	return l.buildDocker(ctx, dfForExisting, paths, steps, model.Cmd{})
}

func (l *localDockerBuilder) buildDocker(ctx context.Context, df Dockerfile,
	paths []pathMapping, steps []model.Cmd, entrypoint model.Cmd) (digest.Digest, error) {
	baseDigest, err := l.buildFromDfWithFiles(ctx, df, paths)
	if err != nil {
		return "", fmt.Errorf("buildFromDfWithFiles: %v", err)
	}

	// Start a container from the base image so we can run steps on it.
	cID, err := l.pool.claimContainer(ctx, baseDigest)

	for _, s := range steps {
		err := l.execInContainer(ctx, cID, s)
		if err != nil {
			return "", fmt.Errorf("buildDocker#step(%v): %v", s, err)
		}
	}

	newDigest, err := l.pool.commitContainer(ctx, cID, entrypoint)
	if err != nil {
		return "", fmt.Errorf("buildDocker: %v", err)
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

	defer func() {
		err := imagePushResponse.Close()
		if err != nil {
			logger.Get(ctx).Info("unable to close imagePushResponse: %s", err)
		}
	}()
	pushedDigest, err := getDigestFromPushOutput(imagePushResponse)
	if err != nil {
		return "", fmt.Errorf("PushDocker#getDigestFromPushOutput: %v", err)
	}

	return pushedDigest, nil
}

func (l *localDockerBuilder) buildFromDfWithFiles(ctx context.Context, df Dockerfile, paths []pathMapping) (digest.Digest, error) {
	err := df.Validate()
	if err != nil {
		return "", err
	}

	archive, err := TarContextAndUpdateDf(df, paths)
	if err != nil {
		return "", err
	}

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

	defer func() {
		err := imageBuildResponse.Body.Close()
		if err != nil {
			logger.Get(ctx).Info("unable to close imagePushResponse: %s", err)
		}
	}()
	result, err := readDockerOutput(imageBuildResponse.Body)
	if err != nil {
		return "", fmt.Errorf("ImageBuild: %v", err)
	}

	return getDigestFromAux(*result)
}

func (l *localDockerBuilder) execInContainer(ctx context.Context, cID containerID, cmd model.Cmd) error {
	created, err := l.dcli.ContainerExecCreate(ctx, string(cID), types.ExecConfig{
		Cmd: cmd.Argv,
	})
	if err != nil {
		return err
	}

	execID := execID(created.ID)
	if execID == "" {
		return fmt.Errorf("execInContainer: failed to create")
	}

	attached, err := l.dcli.ContainerExecAttach(ctx, string(execID), types.ExecStartCheck{})
	if err != nil {
		return fmt.Errorf("execInContainer#attach: %v", err)
	}
	defer attached.Close()

	// TODO(nick): feed this reader into the logger
	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, attached.Reader)
	if err != nil {
		return fmt.Errorf("execInContainer#copy: %v", err)
	}

	// Poll docker until the daemon collects the exit code of the process.
	// This may happen after it collects the stdout above.
	for true {
		inspected, err := l.dcli.ContainerExecInspect(ctx, string(execID))
		if err != nil {
			return fmt.Errorf("execInContainer#inspect: %v", err)
		}

		if inspected.Running {
			continue
		}

		status := inspected.ExitCode
		if status != 0 {
			return fmt.Errorf("Failed with exit code %d. Output:\n%s", status, buf.String())
		}
		return nil
	}
	return nil
}

// startContainer starts a container from the given config.
// Returns the container id iff the container successfully runs; else, error.
//
// TODO(nick): Remove this function. It's currently only used in tests.
// It will be better to use the container pool with exec.
func (l *localDockerBuilder) startContainer(ctx context.Context, config *container.Config) (cId string, err error) {
	resp, err := l.dcli.ContainerCreate(ctx, config, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("startContainer: %v", err)
	}
	containerID := resp.ID

	err = l.dcli.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
	if err != nil {
		return "", fmt.Errorf("startContainer: %v", err)
	}

	statusCh, errCh := l.dcli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return "", fmt.Errorf("startContainer: %v", err)
		}
	case status := <-statusCh:
		if status.Error != nil {
			return "", fmt.Errorf("startContainer: %v", status.Error.Message)
		}
		// TODO(matt) feed this reader into the logger
		r, err := l.dcli.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
		if err != nil {
			return "", fmt.Errorf("startContainer: %v", err)
		}
		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(r)
		if err != nil {
			return "", fmt.Errorf("startContainer: %v", err)
		}
		if status.StatusCode != 0 {
			return "", fmt.Errorf("container '%+v' had non-0 exit code %v. output: '%v'", config, status.StatusCode, string(buf.Bytes()))
		}
	}

	return containerID, nil
}

func TarContextAndUpdateDf(df Dockerfile, paths []pathMapping) (*bytes.Reader, error) {
	buf, err := tarContextAndUpdateDf(df, paths)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(buf.Bytes()), nil
}

func tarContextAndUpdateDf(df Dockerfile, paths []pathMapping) (*bytes.Buffer, error) {
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

func updateDf(df Dockerfile, dnePaths []pathMapping) Dockerfile {
	// Add 'ADD' statements (right now just add whatever's in context;
	// this is safe b/c only adds/overwrites, doesn't remove).
	newDf := df.AddAll()

	return newDf.RmPaths(dnePaths)
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

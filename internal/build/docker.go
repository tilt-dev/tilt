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

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"

	"github.com/docker/cli/cli/command"
	cliflags "github.com/docker/cli/cli/flags"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/registry"
	"github.com/opencontainers/go-digest"
)

// Tilt always pushes to the same tag. We never push to latest.
const tiltTag = "wm-tilt"

var ErrEntrypointInDockerfile = errors.New("base Dockerfile contains an ENTRYPOINT, " +
	"which is not currently supported -- provide an entrypoint in your Tiltfile")

type localDockerBuilder struct {
	dcli *client.Client
}

type Builder interface {
	BuildDockerFromScratch(ctx context.Context, baseDockerfile Dockerfile, mounts []model.Mount, steps []model.Cmd, entrypoint model.Cmd) (digest.Digest, error)
	BuildDockerFromExisting(ctx context.Context, existing digest.Digest, paths []pathMapping, steps []model.Cmd) (digest.Digest, error)
	PushDocker(ctx context.Context, name reference.Named, dig digest.Digest) (reference.Canonical, error)
	TagDocker(ctx context.Context, name reference.Named, dig digest.Digest) (reference.Canonical, error)
}

type pushOutput struct {
	Tag    string
	Digest string
	Size   int
}

var _ Builder = &localDockerBuilder{}

func NewLocalDockerBuilder(dcli *client.Client) *localDockerBuilder {
	return &localDockerBuilder{dcli: dcli}
}

func (l *localDockerBuilder) BuildDockerFromScratch(ctx context.Context, baseDockerfile Dockerfile,
	mounts []model.Mount, steps []model.Cmd, entrypoint model.Cmd) (digest.Digest, error) {
	logger.Get(ctx).Verbose("- Building Docker image from scratch")

	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-BuildDockerFromScratch")
	defer span.Finish()

	return l.buildDocker(ctx, baseDockerfile, MountsToPath(mounts), steps, entrypoint)
}

func (l *localDockerBuilder) BuildDockerFromExisting(ctx context.Context, existing digest.Digest,
	paths []pathMapping, steps []model.Cmd) (digest.Digest, error) {
	logger.Get(ctx).Verbose("- Building Docker image from existing image: %s", existing.Encoded()[:10])

	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-BuildDockerFromExisting")
	defer span.Finish()

	dfForExisting := DockerfileFromExisting(existing)
	return l.buildDocker(ctx, dfForExisting, paths, steps, model.Cmd{})
}

func (l *localDockerBuilder) buildDocker(ctx context.Context, df Dockerfile,
	paths []pathMapping, steps []model.Cmd, entrypoint model.Cmd) (digest.Digest, error) {
	// Do a docker build with the new files.
	resultDigest, err := l.buildFromDfWithFiles(ctx, df, paths)
	if err != nil {
		return "", fmt.Errorf("buildFromDfWithFiles: %v", err)
	}

	if len(steps) > 0 || !entrypoint.Empty() {
		// Do a separate docker build with the new steps.
		stepsDf := DockerfileFromExisting(resultDigest)
		for _, step := range steps {
			stepsDf = stepsDf.Run(step)
		}

		if !entrypoint.Empty() {
			stepsDf = stepsDf.Entrypoint(entrypoint)
		}

		resultDigest, err = l.buildWithDfOnly(ctx, stepsDf)
		if err != nil {
			return "", fmt.Errorf("buildWithDfOnly: %v", err)
		}
	}

	return resultDigest, nil
}

// Tag the digest with the given name and wm-tilt tag.
func (l *localDockerBuilder) TagDocker(ctx context.Context, ref reference.Named, dig digest.Digest) (reference.Canonical, error) {
	tag, err := reference.WithTag(ref, tiltTag)
	if err != nil {
		return nil, fmt.Errorf("TaggDocker: %v", err)
	}

	err = l.dcli.ImageTag(ctx, dig.String(), tag.String())
	if err != nil {
		return nil, fmt.Errorf("TagDocker#ImageTag: %v", err)
	}

	return reference.WithDigest(tag, dig)
}

// Naively tag the digest and push it up to the docker registry specified in the name.
//
// TODO(nick) In the future, I would like us to be smarter about checking if the kubernetes cluster
// we're running in has access to the given registry. And if it doesn't, we should either emit an
// error, or push to a registry that kubernetes does have access to (e.g., a local registry).
func (l *localDockerBuilder) PushDocker(ctx context.Context, ref reference.Named, dig digest.Digest) (reference.Canonical, error) {
	logger.Get(ctx).Verbose("- Pushing Docker image")

	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-PushDocker")
	defer span.Finish()

	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return nil, fmt.Errorf("PushDocker#ParseRepositoryInfo: %s", err)
	}

	logger.Get(ctx).Verbose("-- connecting to repository")
	writer := logger.Get(ctx).Writer(logger.VerboseLvl)
	cli := command.NewDockerCli(nil, writer, writer, true)
	err = cli.Initialize(cliflags.NewClientOptions())
	if err != nil {
		return nil, fmt.Errorf("PushDocker#InitializeCLI: %s", err)
	}
	authConfig := command.ResolveAuthConfig(ctx, cli, repoInfo.Index)
	requestPrivilege := command.RegistryAuthenticationPrivilegedFunc(cli, repoInfo.Index, "push")

	encodedAuth, err := command.EncodeAuthToBase64(authConfig)
	if err != nil {
		return nil, fmt.Errorf("PushDocker#EncodeAuthToBase64: %s", err)
	}

	options := types.ImagePushOptions{
		RegistryAuth:  encodedAuth,
		PrivilegeFunc: requestPrivilege,
	}

	if reference.Domain(ref) == "" {
		return nil, fmt.Errorf("PushDocker: no domain in container name: %s", ref)
	}

	tag, err := reference.WithTag(ref, tiltTag)
	if err != nil {
		return nil, fmt.Errorf("PushDocker: %v", err)
	}

	err = l.dcli.ImageTag(ctx, dig.String(), tag.String())
	if err != nil {
		return nil, fmt.Errorf("PushDocker#ImageTag: %v", err)
	}

	logger.Get(ctx).Verbose("-- pushing the image")
	imagePushResponse, err := l.dcli.ImagePush(
		ctx,
		tag.String(),
		options)
	if err != nil {
		return nil, fmt.Errorf("PushDocker#ImagePush: %v", err)
	}

	defer func() {
		err := imagePushResponse.Close()
		if err != nil {
			logger.Get(ctx).Info("unable to close imagePushResponse: %s", err)
		}
	}()
	pushedDigest, err := getDigestFromPushOutput(ctx, imagePushResponse)
	if err != nil {
		return nil, fmt.Errorf("PushDocker#getDigestFromPushOutput: %v", err)
	}

	return reference.WithDigest(tag, pushedDigest)
}

// TODO(nick): Unify this with buildFromDfWithFiles
func (l *localDockerBuilder) buildWithDfOnly(ctx context.Context, df Dockerfile) (digest.Digest, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-buildWithDfOnly")
	defer span.Finish()

	archive, err := TarWithDfOnly(ctx, df)
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
			logger.Get(ctx).Info("unable to close imageBuildResponse: %s", err)
		}
	}()
	result, err := readDockerOutput(ctx, imageBuildResponse.Body)
	if err != nil {
		return "", fmt.Errorf("ImageBuild: %v", err)
	}

	return getDigestFromAux(*result)
}

func (l *localDockerBuilder) buildFromDfWithFiles(ctx context.Context, df Dockerfile, paths []pathMapping) (digest.Digest, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-buildFromDfWithFiles")
	defer span.Finish()
	err := df.ForbidEntrypoint()
	if err != nil {
		return "", err
	}

	logger.Get(ctx).Verbose("-- tarring context")

	archive, err := TarContextAndUpdateDf(ctx, df, paths)
	if err != nil {
		return "", err
	}

	logger.Get(ctx).Verbose("-- building image")
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
			logger.Get(ctx).Info("unable to close imageBuildResponse: %s", err)
		}
	}()
	result, err := readDockerOutput(ctx, imageBuildResponse.Body)
	if err != nil {
		return "", fmt.Errorf("ImageBuild: %v", err)
	}

	return getDigestFromAux(*result)
}

func (l *localDockerBuilder) execInContainer(ctx context.Context, cID containerID, cmd model.Cmd) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-execInContainer")
	defer span.Finish()
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

// TODO(nick): Unify this with TarContextAndUpdateDf
func TarWithDfOnly(ctx context.Context, df Dockerfile) (*bytes.Reader, error) {
	buf, err := tarWithDfOnly(ctx, df)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(buf.Bytes()), nil
}

func TarContextAndUpdateDf(ctx context.Context, df Dockerfile, paths []pathMapping) (*bytes.Reader, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-TarContextAndUpdateDf")
	defer span.Finish()
	buf, err := tarContextAndUpdateDf(df, paths)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(buf.Bytes()), nil
}

// TODO(nick): Unify this with tarContextAndUpdateDf
func tarWithDfOnly(ctx context.Context, df Dockerfile) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer func() {
		err := tw.Close()
		if err != nil {
			log.Printf("Error closing tar writer: %s", err.Error())
		}
	}()

	err := archiveDf(tw, df)
	if err != nil {
		return nil, fmt.Errorf("archiveDf: %v", err)
	}

	return buf, nil
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
func readDockerOutput(ctx context.Context, reader io.Reader) (*json.RawMessage, error) {
	var result *json.RawMessage
	decoder := json.NewDecoder(reader)
	for decoder.More() {
		message := jsonmessage.JSONMessage{}
		err := decoder.Decode(&message)
		if err != nil {
			return nil, fmt.Errorf("decoding docker output: %v", err)
		}

		// TODO(Han): make me smarter! 🤓
		logger.Get(ctx).Info("%+v\n", message)

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

func getDigestFromBuildOutput(ctx context.Context, reader io.Reader) (digest.Digest, error) {
	aux, err := readDockerOutput(ctx, reader)
	if err != nil {
		return "", err
	}
	if aux == nil {
		return "", fmt.Errorf("getDigestFromBuildOutput: No results found in docker output")
	}
	return getDigestFromAux(*aux)
}

func getDigestFromPushOutput(ctx context.Context, reader io.Reader) (digest.Digest, error) {
	aux, err := readDockerOutput(ctx, reader)
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

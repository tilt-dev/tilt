package build

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/output"

	"github.com/docker/cli/cli/command"
	cliflags "github.com/docker/cli/cli/flags"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/registry"
	"github.com/opencontainers/go-digest"
)

var ErrEntrypointInDockerfile = errors.New("base Dockerfile contains an ENTRYPOINT, " +
	"which is not currently supported -- provide an entrypoint in your Tiltfile")

type localDockerBuilder struct {
	dcli DockerClient
}

type Builder interface {
	BuildDockerFromScratch(ctx context.Context, ref reference.Named, baseDockerfile Dockerfile, mounts []model.Mount, steps []model.Cmd, entrypoint model.Cmd) (reference.NamedTagged, error)
	BuildDockerFromExisting(ctx context.Context, existing reference.NamedTagged, paths []pathMapping, steps []model.Cmd) (reference.NamedTagged, error)
	PushDocker(ctx context.Context, name reference.NamedTagged) (reference.NamedTagged, error)
	TagDocker(ctx context.Context, name reference.Named, dig digest.Digest) (reference.NamedTagged, error)
}

func DefaultBuilder(b *localDockerBuilder) Builder {
	return b
}

type pushOutput struct {
	Tag    string
	Digest string
	Size   int
}

var _ Builder = &localDockerBuilder{}

func NewLocalDockerBuilder(dcli DockerClient) *localDockerBuilder {
	return &localDockerBuilder{dcli: dcli}
}

func (l *localDockerBuilder) BuildDockerFromScratch(ctx context.Context, ref reference.Named, baseDockerfile Dockerfile,
	mounts []model.Mount, steps []model.Cmd, entrypoint model.Cmd) (reference.NamedTagged, error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-BuildDockerFromScratch")
	defer span.Finish()

	err := baseDockerfile.ForbidEntrypoint()
	if err != nil {
		return nil, err
	}

	return l.buildDocker(ctx, baseDockerfile, MountsToPathMappings(mounts), steps, entrypoint, ref)
}

func (l *localDockerBuilder) BuildDockerFromExisting(ctx context.Context, existing reference.NamedTagged,
	paths []pathMapping, steps []model.Cmd) (reference.NamedTagged, error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-BuildDockerFromExisting")
	defer span.Finish()

	dfForExisting := DockerfileFromExisting(existing)
	return l.buildDocker(ctx, dfForExisting, paths, steps, model.Cmd{}, existing)
}

func (l *localDockerBuilder) buildDocker(ctx context.Context, baseDockerfile Dockerfile,
	paths []pathMapping, steps []model.Cmd, entrypoint model.Cmd, ref reference.Named) (reference.NamedTagged, error) {

	df := baseDockerfile.AddAll()
	toRemove, err := missingLocalPaths(ctx, paths)
	if err != nil {
		return nil, fmt.Errorf("buildDocker: %v", err)
	}

	df = df.RmPaths(toRemove)

	for _, step := range steps {
		df = df.Run(step)
	}

	if !entrypoint.Empty() {
		df = df.Entrypoint(entrypoint)
	}

	// We have the Dockerfile! Kick off the docker build.
	namedTagged, err := l.buildFromDf(ctx, df, paths, ref)
	if err != nil {
		return nil, fmt.Errorf("buildDocker#buildFromDf: %v", err)
	}

	return namedTagged, nil
}

// Tag the digest with the given name and wm-tilt tag.
func (l *localDockerBuilder) TagDocker(ctx context.Context, ref reference.Named, dig digest.Digest) (reference.NamedTagged, error) {
	tag, err := digestAsTag(dig)
	if err != nil {
		return nil, fmt.Errorf("TagDocker: %v", err)
	}

	namedTagged, err := reference.WithTag(ref, tag)
	if err != nil {
		return nil, fmt.Errorf("TagDocker: %v", err)
	}

	err = l.dcli.ImageTag(ctx, dig.String(), namedTagged.String())
	if err != nil {
		return nil, fmt.Errorf("TagDocker#ImageTag: %v", err)
	}

	return namedTagged, nil
}

// Naively tag the digest and push it up to the docker registry specified in the name.
//
// TODO(nick) In the future, I would like us to be smarter about checking if the kubernetes cluster
// we're running in has access to the given registry. And if it doesn't, we should either emit an
// error, or push to a registry that kubernetes does have access to (e.g., a local registry).
func (l *localDockerBuilder) PushDocker(ctx context.Context, ref reference.NamedTagged) (reference.NamedTagged, error) {
	logger.Get(ctx).Infof("Pushing Docker image")

	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-PushDocker")
	defer span.Finish()

	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return nil, fmt.Errorf("PushDocker#ParseRepositoryInfo: %s", err)
	}

	logger.Get(ctx).Infof("%sconnecting to repository", logger.Tab)
	writer := output.Get(ctx).Writer()
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

	logger.Get(ctx).Infof("%spushing the image", logger.Tab)
	imagePushResponse, err := l.dcli.ImagePush(
		ctx,
		ref.String(),
		options)
	if err != nil {
		return nil, fmt.Errorf("PushDocker#ImagePush: %v", err)
	}

	defer func() {
		err := imagePushResponse.Close()
		if err != nil {
			logger.Get(ctx).Infof("unable to close imagePushResponse: %s", err)
		}
	}()
	_, err = getDigestFromPushOutput(ctx, imagePushResponse)
	if err != nil {
		return nil, fmt.Errorf("PushDocker#getDigestFromPushOutput: %v", err)
	}

	return ref, nil
}

func (l *localDockerBuilder) buildFromDf(ctx context.Context, df Dockerfile, paths []pathMapping, ref reference.Named) (reference.NamedTagged, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-buildFromDf")
	defer span.Finish()

	output.Get(ctx).StartBuildStep("tarring context")

	archive, err := TarContextAndUpdateDf(ctx, df, paths)
	if err != nil {
		return nil, err
	}

	output.Get(ctx).StartBuildStep("building image")
	spanBuild, ctx := opentracing.StartSpanFromContext(ctx, "daemon-ImageBuild")
	imageBuildResponse, err := l.dcli.ImageBuild(
		ctx,
		archive,
		Options(archive),
	)
	spanBuild.Finish()
	if err != nil {
		return nil, err
	}

	defer func() {
		err := imageBuildResponse.Body.Close()
		if err != nil {
			logger.Get(ctx).Infof("unable to close imagePushResponse: %s", err)
		}
	}()
	result, err := readDockerOutput(ctx, imageBuildResponse.Body)
	if err != nil {
		return nil, fmt.Errorf("ImageBuild: %v", err)
	}

	digest, err := getDigestFromAux(*result)
	if err != nil {
		return nil, fmt.Errorf("getDigestFromAux: %v", err)
	}

	nt, err := l.TagDocker(ctx, ref, digest)
	if err != nil {
		return nil, fmt.Errorf("PushDocker: %v", err)
	}

	return nt, nil
}

func TarContextAndUpdateDf(ctx context.Context, df Dockerfile, paths []pathMapping) (*bytes.Reader, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-TarContextAndUpdateDf")
	defer span.Finish()
	buf, err := tarContextAndUpdateDf(ctx, df, paths)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(buf.Bytes()), nil
}

func tarContextAndUpdateDf(ctx context.Context, df Dockerfile, paths []pathMapping) (*bytes.Buffer, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-tarContextAndUpdateDf")
	defer span.Finish()
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer func() {
		err := tw.Close()
		if err != nil {
			log.Printf("Error closing tar writer: %s", err.Error())
		}
	}()

	// TODO: maybe write our own tarWriter struct with methods on it, so it's clearer that we're modifying the tar writer in place
	err := archivePaths(ctx, tw, paths)
	if err != nil {
		return nil, fmt.Errorf("archiveIfExists: %v", err)
	}

	err = archiveDf(ctx, tw, df)
	if err != nil {
		return nil, fmt.Errorf("archiveDf: %v", err)
	}

	return buf, nil
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
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-readDockerOutput")
	defer span.Finish()
	var result *json.RawMessage
	decoder := json.NewDecoder(reader)
	var innerSpan opentracing.Span
	for decoder.More() {
		if innerSpan != nil {
			innerSpan.Finish()
		}
		message := jsonmessage.JSONMessage{}
		err := decoder.Decode(&message)
		if err != nil {
			return nil, fmt.Errorf("decoding docker output: %v", err)
		}

		if len(message.Stream) > 0 && message.Stream != "\n" {
			msg := strings.TrimSuffix(message.Stream, "\n")
			output.Get(ctx).Print("%+v", msg)
			if strings.HasPrefix(msg, "Step") || strings.HasPrefix(msg, "Running") {
				innerSpan, ctx = opentracing.StartSpanFromContext(ctx, msg)
			}
		}

		if message.ErrorMessage != "" {
			return nil, errors.New(message.ErrorMessage)
		}

		if message.Error != nil {
			return nil, errors.New(message.Error.Message)
		}

		if message.Aux != nil && message.ID != "moby.buildkit.trace" {
			result = message.Aux
		}
	}
	if innerSpan != nil {
		innerSpan.Finish()
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

	if aux == nil {
		return "", fmt.Errorf("No digest found in push output")
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

func digestAsTag(d digest.Digest) (string, error) {
	str := d.Encoded()
	if len(str) < 16 {
		return "", fmt.Errorf("Digest too short: %s", str)
	}
	return fmt.Sprintf("tilt-%s", str[:16]), nil
}

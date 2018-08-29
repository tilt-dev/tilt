package build

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/containerd/console"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/util/progress/progressui"
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

type dockerImageBuilder struct {
	dcli    DockerClient
	console console.Console
	out     io.Writer
}

type ImageBuilder interface {
	BuildImageFromScratch(ctx context.Context, ref reference.Named, baseDockerfile Dockerfile, mounts []model.Mount, steps []model.Cmd, entrypoint model.Cmd) (reference.NamedTagged, error)
	BuildImageFromExisting(ctx context.Context, existing reference.NamedTagged, paths []pathMapping, steps []model.Cmd) (reference.NamedTagged, error)
	PushImage(ctx context.Context, name reference.NamedTagged) (reference.NamedTagged, error)
	TagImage(ctx context.Context, name reference.Named, dig digest.Digest) (reference.NamedTagged, error)
}

func DefaultImageBuilder(b *dockerImageBuilder) ImageBuilder {
	return b
}

func DefaultConsole() console.Console {
	out := os.Stdout
	c, _ := console.ConsoleFromFile(out)

	return c
}

func DefaultOut() io.Writer {
	return os.Stdout
}

type pushOutput struct {
	Tag    string
	Digest string
	Size   int
}

var _ ImageBuilder = &dockerImageBuilder{}

func NewDockerImageBuilder(dcli DockerClient, console console.Console, out io.Writer) *dockerImageBuilder {
	return &dockerImageBuilder{dcli: dcli, console: console, out: out}
}

func (d *dockerImageBuilder) BuildImageFromScratch(ctx context.Context, ref reference.Named, baseDockerfile Dockerfile,
	mounts []model.Mount, steps []model.Cmd, entrypoint model.Cmd) (reference.NamedTagged, error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-BuildImageFromScratch")
	defer span.Finish()

	err := baseDockerfile.ValidateBaseDockerfile()
	if err != nil {
		return nil, err
	}

	return d.buildImage(ctx, baseDockerfile, MountsToPathMappings(mounts), steps, entrypoint, ref)
}

func (d *dockerImageBuilder) BuildImageFromExisting(ctx context.Context, existing reference.NamedTagged,
	paths []pathMapping, steps []model.Cmd) (reference.NamedTagged, error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-BuildImageFromExisting")
	defer span.Finish()

	dfForExisting := DockerfileFromExisting(existing)
	return d.buildImage(ctx, dfForExisting, paths, steps, model.Cmd{}, existing)
}

func (d *dockerImageBuilder) buildImage(ctx context.Context, baseDockerfile Dockerfile,
	paths []pathMapping, steps []model.Cmd, entrypoint model.Cmd, ref reference.Named) (reference.NamedTagged, error) {

	df := baseDockerfile.AddAll()
	toRemove, err := missingLocalPaths(ctx, paths)
	if err != nil {
		return nil, fmt.Errorf("buildImage: %v", err)
	}

	df = df.RmPaths(toRemove)

	for _, step := range steps {
		df = df.Run(step)
	}

	if !entrypoint.Empty() {
		df = df.Entrypoint(entrypoint)
	}

	// We have the Dockerfile! Kick off the docker build.
	namedTagged, err := d.buildFromDf(ctx, df, paths, ref)
	if err != nil {
		return nil, fmt.Errorf("buildImage#buildFromDf: %v", err)
	}

	return namedTagged, nil
}

// Tag the digest with the given name and wm-tilt tag.
func (d *dockerImageBuilder) TagImage(ctx context.Context, ref reference.Named, dig digest.Digest) (reference.NamedTagged, error) {
	tag, err := digestAsTag(dig)
	if err != nil {
		return nil, fmt.Errorf("TagImage: %v", err)
	}

	namedTagged, err := reference.WithTag(ref, tag)
	if err != nil {
		return nil, fmt.Errorf("TagImage: %v", err)
	}

	err = d.dcli.ImageTag(ctx, dig.String(), namedTagged.String())
	if err != nil {
		return nil, fmt.Errorf("TagImage#ImageTag: %v", err)
	}

	return namedTagged, nil
}

// Naively tag the digest and push it up to the docker registry specified in the name.
//
// TODO(nick) In the future, I would like us to be smarter about checking if the kubernetes cluster
// we're running in has access to the given registry. And if it doesn't, we should either emit an
// error, or push to a registry that kubernetes does have access to (e.g., a local registry).
func (d *dockerImageBuilder) PushImage(ctx context.Context, ref reference.NamedTagged) (reference.NamedTagged, error) {
	logger.Get(ctx).Infof("Pushing Docker image")

	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-PushImage")
	defer span.Finish()

	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return nil, fmt.Errorf("PushImage#ParseRepositoryInfo: %s", err)
	}

	logger.Get(ctx).Infof("%sconnecting to repository", logger.Tab)
	writer := output.Get(ctx).Writer()
	cli := command.NewDockerCli(nil, writer, writer, true)

	err = cli.Initialize(cliflags.NewClientOptions())
	if err != nil {
		return nil, fmt.Errorf("PushImage#InitializeCLI: %s", err)
	}
	authConfig := command.ResolveAuthConfig(ctx, cli, repoInfo.Index)
	requestPrivilege := command.RegistryAuthenticationPrivilegedFunc(cli, repoInfo.Index, "push")

	encodedAuth, err := command.EncodeAuthToBase64(authConfig)
	if err != nil {
		return nil, fmt.Errorf("PushImage#EncodeAuthToBase64: %s", err)
	}

	options := types.ImagePushOptions{
		RegistryAuth:  encodedAuth,
		PrivilegeFunc: requestPrivilege,
	}

	if reference.Domain(ref) == "" {
		return nil, fmt.Errorf("PushImage: no domain in container name: %s", ref)
	}

	logger.Get(ctx).Infof("%spushing the image", logger.Tab)
	imagePushResponse, err := d.dcli.ImagePush(
		ctx,
		ref.String(),
		options)
	if err != nil {
		return nil, fmt.Errorf("PushImage#ImagePush: %v", err)
	}

	defer func() {
		err := imagePushResponse.Close()
		if err != nil {
			logger.Get(ctx).Infof("unable to close imagePushResponse: %s", err)
		}
	}()
	_, err = d.getDigestFromPushOutput(ctx, imagePushResponse)
	if err != nil {
		return nil, fmt.Errorf("PushImage#getDigestFromPushOutput: %v", err)
	}

	return ref, nil
}

func (d *dockerImageBuilder) buildFromDf(ctx context.Context, df Dockerfile, paths []pathMapping, ref reference.Named) (reference.NamedTagged, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-buildFromDf")
	defer span.Finish()

	output.Get(ctx).StartBuildStep("tarring context")

	archive, err := TarContextAndUpdateDf(ctx, df, paths)
	if err != nil {
		return nil, err
	}

	output.Get(ctx).StartBuildStep("building image")
	spanBuild, ctx := opentracing.StartSpanFromContext(ctx, "daemon-ImageBuild")
	imageBuildResponse, err := d.dcli.ImageBuild(
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
	result, err := d.readDockerOutput(ctx, imageBuildResponse.Body)
	if err != nil {
		return nil, fmt.Errorf("ImageBuild: %v", err)
	}
	if result == nil {
		return nil, fmt.Errorf("Unable to read docker output: result is nil")
	}

	digest, err := getDigestFromAux(*result)
	if err != nil {
		return nil, fmt.Errorf("getDigestFromAux: %v", err)
	}

	nt, err := d.TagImage(ctx, ref, digest)
	if err != nil {
		return nil, fmt.Errorf("PushImage: %v", err)
	}

	return nt, nil
}

func TarContextAndUpdateDf(ctx context.Context, df Dockerfile, paths []pathMapping) (*bytes.Reader, error) {
	buf, err := tarContextAndUpdateDf(ctx, df, paths)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(buf.Bytes()), nil
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
func (d *dockerImageBuilder) readDockerOutput(ctx context.Context, reader io.Reader) (*json.RawMessage, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-readDockerOutput")
	defer span.Finish()

	displayCh := make(chan *client.SolveStatus)
	defer close(displayCh)

	displayStatus := func(displayCh chan *client.SolveStatus) {
		go func() {
			err := progressui.DisplaySolveStatus(ctx, "", d.console, d.out, displayCh)
			if err != nil {
				output.Get(ctx).Printf("Error printing progressui: %s", err)
			}
		}()
	}

	displayStatus(displayCh)

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
			output.Get(ctx).Printf("%+v", msg)
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

		if messageIsFromBuildkit(message) {
			var resp controlapi.StatusResponse
			var dt []byte
			// ignoring all messages that are not understood
			if err := json.Unmarshal(*message.Aux, &dt); err != nil {
				return nil, err
			}
			if err := (&resp).Unmarshal(dt); err != nil {
				return nil, err
			}

			s := client.SolveStatus{}
			for _, v := range resp.Vertexes {
				s.Vertexes = append(s.Vertexes, &client.Vertex{
					Digest:    v.Digest,
					Inputs:    v.Inputs,
					Name:      v.Name,
					Started:   v.Started,
					Completed: v.Completed,
					Error:     v.Error,
					Cached:    v.Cached,
				})
			}
			for _, v := range resp.Statuses {
				s.Statuses = append(s.Statuses, &client.VertexStatus{
					ID:        v.ID,
					Vertex:    v.Vertex,
					Name:      v.Name,
					Total:     v.Total,
					Current:   v.Current,
					Timestamp: v.Timestamp,
					Started:   v.Started,
					Completed: v.Completed,
				})
			}
			for _, v := range resp.Logs {
				s.Logs = append(s.Logs, &client.VertexLog{
					Vertex:    v.Vertex,
					Stream:    int(v.Stream),
					Data:      v.Msg,
					Timestamp: v.Timestamp,
				})
			}

			displayCh <- &s
		}

		if message.Aux != nil && !messageIsFromBuildkit(message) {
			result = message.Aux
		}
	}
	if innerSpan != nil {
		innerSpan.Finish()
	}
	return result, nil
}

func messageIsFromBuildkit(msg jsonmessage.JSONMessage) bool {
	return msg.ID == "moby.buildkit.trace"
}

func (d *dockerImageBuilder) getDigestFromBuildOutput(ctx context.Context, reader io.Reader) (digest.Digest, error) {
	aux, err := d.readDockerOutput(ctx, reader)
	if err != nil {
		return "", err
	}
	if aux == nil {
		return "", fmt.Errorf("getDigestFromBuildOutput: No results found in docker output")
	}
	return getDigestFromAux(*aux)
}

func (d *dockerImageBuilder) getDigestFromPushOutput(ctx context.Context, reader io.Reader) (digest.Digest, error) {
	aux, err := d.readDockerOutput(ctx, reader)
	if err != nil {
		return "", err
	}

	if aux == nil {
		return "", fmt.Errorf("no digest found in push output")
	}

	dig := pushOutput{}
	err = json.Unmarshal(*aux, &dig)
	if err != nil {
		return "", fmt.Errorf("getDigestFromPushOutput#Unmarshal: %v, json string: %+v", err, aux)
	}

	if dig.Digest == "" {
		return "", fmt.Errorf("getDigestFromPushOutput: Digest not found in %+v", aux)
	}

	return digest.Digest(dig.Digest), nil
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

package build

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/dustin/go-humanize"

	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/dockerfile"
	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"

	"github.com/docker/cli/cli/command"
	cliflags "github.com/docker/cli/cli/flags"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/registry"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/opencontainers/go-digest"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
)

type dockerImageBuilder struct {
	dCli docker.Client

	// A set of extra labels to attach to all builds
	// created by this image builder.
	//
	// By default, all builds are labeled with a build mode.
	extraLabels dockerfile.Labels
}

type ImageBuilder interface {
	BuildDockerfile(ctx context.Context, ps *PipelineState, ref reference.Named, df dockerfile.Dockerfile, buildPath string, filter model.PathMatcher, buildArgs map[string]string) (reference.NamedTagged, error)
	BuildImageFromScratch(ctx context.Context, ps *PipelineState, ref reference.Named, baseDockerfile dockerfile.Dockerfile, mounts []model.Mount, filter model.PathMatcher, steps []model.Step, entrypoint model.Cmd) (reference.NamedTagged, error)
	BuildImageFromExisting(ctx context.Context, ps *PipelineState, existing reference.NamedTagged, paths []PathMapping, filter model.PathMatcher, steps []model.Step) (reference.NamedTagged, error)
	PushImage(ctx context.Context, name reference.NamedTagged, writer io.Writer) (reference.NamedTagged, error)
	TagImage(ctx context.Context, name reference.Named, dig digest.Digest) (reference.NamedTagged, error)
}

func DefaultImageBuilder(b *dockerImageBuilder) ImageBuilder {
	return b
}

var _ ImageBuilder = &dockerImageBuilder{}

func NewDockerImageBuilder(dCli docker.Client, extraLabels dockerfile.Labels) *dockerImageBuilder {
	return &dockerImageBuilder{
		dCli:        dCli,
		extraLabels: extraLabels,
	}
}

func (d *dockerImageBuilder) BuildDockerfile(ctx context.Context, ps *PipelineState, ref reference.Named, df dockerfile.Dockerfile, buildPath string, filter model.PathMatcher, buildArgs map[string]string) (reference.NamedTagged, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "dib-BuildDockerfile")
	defer span.Finish()

	paths := []PathMapping{
		{
			LocalPath:     buildPath,
			ContainerPath: "/",
		},
	}
	return d.buildFromDf(ctx, ps, df, paths, filter, ref, buildArgs)
}

func (d *dockerImageBuilder) BuildImageFromScratch(ctx context.Context, ps *PipelineState, ref reference.Named, baseDockerfile dockerfile.Dockerfile,
	mounts []model.Mount, filter model.PathMatcher,
	steps []model.Step, entrypoint model.Cmd) (reference.NamedTagged, error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-BuildImageFromScratch")
	defer span.Finish()

	hasEntrypoint := !entrypoint.Empty()

	paths := MountsToPathMappings(mounts)
	df := baseDockerfile
	df, steps, err := d.addConditionalSteps(df, steps, paths)
	if err != nil {
		return nil, errors.Wrapf(err, "BuildImageFromScratch")
	}

	df = df.AddAll()
	df = d.addRemainingSteps(df, steps)
	if hasEntrypoint {
		df = df.Entrypoint(entrypoint)
	}

	df = d.applyLabels(df, BuildModeScratch)
	return d.buildFromDf(ctx, ps, df, paths, filter, ref, model.DockerBuildArgs{})
}

func (d *dockerImageBuilder) BuildImageFromExisting(ctx context.Context, ps *PipelineState, existing reference.NamedTagged,
	paths []PathMapping, filter model.PathMatcher, steps []model.Step) (reference.NamedTagged, error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-BuildImageFromExisting")
	defer span.Finish()

	df := d.applyLabels(dockerfile.FromExisting(existing), BuildModeExisting)

	// Don't worry about conditional steps on incremental builds, they've
	// already handled by the watch loop.
	df, err := d.addMountsAndRemovedFiles(ctx, df, paths)
	if err != nil {
		return nil, errors.Wrap(err, "BuildImageFromExisting")
	}

	df = d.addRemainingSteps(df, steps)
	return d.buildFromDf(ctx, ps, df, paths, filter, existing, model.DockerBuildArgs{})
}

func (d *dockerImageBuilder) applyLabels(df dockerfile.Dockerfile, buildMode dockerfile.LabelValue) dockerfile.Dockerfile {
	df = df.WithLabel(BuildMode, buildMode)
	for k, v := range d.extraLabels {
		df = df.WithLabel(k, v)
	}
	return df
}

// If the build starts with conditional steps, add the dependent files first,
// then add the runs, before we add the majority of the source.
func (d *dockerImageBuilder) addConditionalSteps(df dockerfile.Dockerfile, steps []model.Step, paths []PathMapping) (dockerfile.Dockerfile, []model.Step, error) {
	consumed := 0
	for _, step := range steps {
		if step.Triggers == nil {
			break
		}

		matcher, err := ignore.CreateStepMatcher(step)
		if err != nil {
			return "", nil, err
		}

		pathsToAdd, err := FilterMappings(paths, matcher)
		if err != nil {
			return "", nil, err
		}

		if len(pathsToAdd) == 0 {
			// TODO(nick): If this happens, it means the input file has been deleted.
			// This seems like a very late part of the pipeline to detect this
			// error. It should have been caught way up when we were evaluating the
			// tiltfile.
			//
			// For now, we're going to return an error to catch this case.
			return "", nil, fmt.Errorf("No inputs for run: %s", step.Cmd)
		}

		for _, p := range pathsToAdd {
			// The tarball root is the same as the container root, so the src and dest
			// are the same.
			df = df.Join(fmt.Sprintf("COPY %s %s", p.ContainerPath, p.ContainerPath))
		}

		// After adding the inputs, run the step.
		//
		// TODO(nick): This assumes that the RUN step doesn't overwrite any input files
		// that might be added later. In that case, we might need to do something
		// clever where we stash the outputs and restore them after the final "ADD . /".
		// But let's see how this works for now.
		df = df.Run(step.Cmd)
		consumed++
	}

	remainingSteps := append([]model.Step{}, steps[consumed:]...)
	return df, remainingSteps, nil
}

func (d *dockerImageBuilder) addMountsAndRemovedFiles(ctx context.Context, df dockerfile.Dockerfile, paths []PathMapping) (dockerfile.Dockerfile, error) {
	df = df.AddAll()
	toRemove, err := MissingLocalPaths(ctx, paths)
	if err != nil {
		return "", errors.Wrap(err, "addMounts")
	}

	toRemovePaths := make([]string, len(toRemove))
	for i, p := range toRemove {
		toRemovePaths[i] = p.ContainerPath
	}

	df = df.RmPaths(toRemovePaths)
	return df, nil
}

func (d *dockerImageBuilder) addRemainingSteps(df dockerfile.Dockerfile, remaining []model.Step) dockerfile.Dockerfile {
	for _, step := range remaining {
		df = df.Run(step.Cmd)
	}
	return df
}

// Tag the digest with the given name and wm-tilt tag.
func (d *dockerImageBuilder) TagImage(ctx context.Context, ref reference.Named, dig digest.Digest) (reference.NamedTagged, error) {
	tag, err := digestAsTag(dig)
	if err != nil {
		return nil, errors.Wrap(err, "TagImage")
	}

	namedTagged, err := reference.WithTag(ref, tag)
	if err != nil {
		return nil, errors.Wrap(err, "TagImage")
	}

	err = d.dCli.ImageTag(ctx, dig.String(), namedTagged.String())
	if err != nil {
		return nil, errors.Wrap(err, "TagImage#ImageTag")
	}

	return namedTagged, nil
}

// Naively tag the digest and push it up to the docker registry specified in the name.
//
// TODO(nick) In the future, I would like us to be smarter about checking if the kubernetes cluster
// we're running in has access to the given registry. And if it doesn't, we should either emit an
// error, or push to a registry that kubernetes does have access to (e.g., a local registry).
func (d *dockerImageBuilder) PushImage(ctx context.Context, ref reference.NamedTagged, writer io.Writer) (reference.NamedTagged, error) {
	l := logger.Get(ctx)
	l.Infof("Pushing Docker image")
	prefix := logger.Blue(l).Sprint("  │ ")

	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-PushImage")
	defer span.Finish()

	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return nil, errors.Wrap(err, "PushImage#ParseRepositoryInfo")
	}

	l.Infof("%sconnecting to repository", prefix)
	cli := command.NewDockerCli(nil, writer, writer, true)

	err = cli.Initialize(cliflags.NewClientOptions())
	if err != nil {
		return nil, errors.Wrap(err, "PushImage#InitializeCLI")
	}
	authConfig := command.ResolveAuthConfig(ctx, cli, repoInfo.Index)
	requestPrivilege := command.RegistryAuthenticationPrivilegedFunc(cli, repoInfo.Index, "push")

	encodedAuth, err := command.EncodeAuthToBase64(authConfig)
	if err != nil {
		return nil, errors.Wrap(err, "PushImage#EncodeAuthToBase64")
	}

	options := types.ImagePushOptions{
		RegistryAuth:  encodedAuth,
		PrivilegeFunc: requestPrivilege,
	}

	if reference.Domain(ref) == "" {
		return nil, errors.Wrap(err, "PushImage: no domain in container name")
	}

	l.Infof("%spushing the image", prefix)
	imagePushResponse, err := d.dCli.ImagePush(
		ctx,
		ref.String(),
		options)
	if err != nil {
		return nil, errors.Wrap(err, "PushImage#ImagePush")
	}

	defer func() {
		err := imagePushResponse.Close()
		if err != nil {
			l.Infof("unable to close imagePushResponse: %s", err)
		}
	}()

	_, err = readDockerOutput(ctx, imagePushResponse, writer)
	if err != nil {
		return nil, errors.Wrapf(err, "pushing image %q", ref.Name())
	}

	return ref, nil
}

func (d *dockerImageBuilder) buildFromDf(ctx context.Context, ps *PipelineState, df dockerfile.Dockerfile, paths []PathMapping, filter model.PathMatcher, ref reference.Named, buildArgs model.DockerBuildArgs) (reference.NamedTagged, error) {
	logger.Get(ctx).Infof("Building Dockerfile:\n%s\n", indent(df.String(), "  "))
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-buildFromDf")
	defer span.Finish()

	// TODO(Han): Extend output to print without newline
	ps.StartBuildStep(ctx, "Tarring context…")

	// NOTE(maia): some people want to know what files we're adding (b/c `ADD . /` isn't descriptive)
	if logger.Get(ctx).Level() >= logger.VerboseLvl {
		for _, pm := range paths {
			ps.Printf(ctx, pm.prettyStr())
		}
	}

	archive, err := tarContextAndUpdateDf(ctx, df, paths, filter)
	if err != nil {
		return nil, err
	}

	// TODO(Han): Extend output to print without newline
	ps.Printf(ctx, "Created tarball (size: %s)",
		humanize.Bytes(uint64(archive.Len())))

	ps.StartBuildStep(ctx, "Building image")
	spanBuild, ctx := opentracing.StartSpanFromContext(ctx, "daemon-ImageBuild")
	imageBuildResponse, err := d.dCli.ImageBuild(
		ctx,
		archive,
		Options(archive, buildArgs),
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

	digest, err := d.getDigestFromBuildOutput(ctx, imageBuildResponse.Body, ps.Writer(ctx))
	if err != nil {
		return nil, err
	}

	nt, err := d.TagImage(ctx, ref, digest)
	if err != nil {
		return nil, errors.Wrap(err, "PushImage")
	}

	return nt, nil
}

func (d *dockerImageBuilder) getDigestFromBuildOutput(ctx context.Context, reader io.Reader, writer io.Writer) (digest.Digest, error) {
	result, err := readDockerOutput(ctx, reader, writer)
	if err != nil {
		return "", errors.Wrap(err, "ImageBuild")
	}

	digest, err := d.getDigestFromDockerOutput(ctx, result)
	if err != nil {
		return "", errors.Wrap(err, "getDigestFromBuildOutput")
	}

	return digest, nil
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
func readDockerOutput(ctx context.Context, reader io.Reader, writer io.Writer) (dockerOutput, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-readDockerOutput")
	defer span.Finish()

	result := dockerOutput{}
	decoder := json.NewDecoder(reader)
	var innerSpan opentracing.Span

	b := newBuildkitPrinter(logger.Get(ctx))

	for decoder.More() {
		if innerSpan != nil {
			innerSpan.Finish()
		}
		message := jsonmessage.JSONMessage{}
		err := decoder.Decode(&message)
		if err != nil {
			return dockerOutput{}, errors.Wrap(err, "decoding docker output")
		}

		if len(message.Stream) > 0 {
			msg := message.Stream

			builtDigestMatch := oldDigestRegexp.FindStringSubmatch(msg)
			if len(builtDigestMatch) >= 2 {
				// Old versions of docker (pre 1.30) didn't send down an aux message.
				result.shortDigest = builtDigestMatch[1]
			}

			_, err = writer.Write([]byte(msg))
			if err != nil {
				return dockerOutput{}, errors.Wrap(err, "failed to write docker output")
			}
			if strings.HasPrefix(msg, "Step") || strings.HasPrefix(msg, "Running") {
				innerSpan, ctx = opentracing.StartSpanFromContext(ctx, msg)
			}
		}

		if message.ErrorMessage != "" {
			return dockerOutput{}, errors.New(message.ErrorMessage)
		}

		if message.Error != nil {
			return dockerOutput{}, errors.New(message.Error.Message)
		}

		if messageIsFromBuildkit(message) {
			err := toBuildkitStatus(message.Aux, b)
			if err != nil {
				return dockerOutput{}, err
			}
		}

		if message.Aux != nil && !messageIsFromBuildkit(message) {
			result.aux = message.Aux
		}
	}

	if innerSpan != nil {
		innerSpan.Finish()
	}
	if ctx.Err() != nil {
		return dockerOutput{}, ctx.Err()
	}
	return result, nil
}

func toBuildkitStatus(aux *json.RawMessage, b *buildkitPrinter) error {
	var resp controlapi.StatusResponse
	var dt []byte
	// ignoring all messages that are not understood
	if err := json.Unmarshal(*aux, &dt); err != nil {
		return err
	}
	if err := (&resp).Unmarshal(dt); err != nil {
		return err
	}

	vertexes := []*vertex{}
	logs := []*vertexLog{}

	for _, v := range resp.Vertexes {
		vertexes = append(vertexes, &vertex{
			digest:    v.Digest,
			name:      v.Name,
			error:     v.Error,
			started:   v.Started != nil,
			completed: v.Completed != nil,
		})
	}
	for _, v := range resp.Logs {
		logs = append(logs, &vertexLog{
			vertex: v.Vertex,
			msg:    v.Msg,
		})
	}

	return b.parseAndPrint(vertexes, logs)
}

func messageIsFromBuildkit(msg jsonmessage.JSONMessage) bool {
	return msg.ID == "moby.buildkit.trace"
}

func (d *dockerImageBuilder) getDigestFromDockerOutput(ctx context.Context, output dockerOutput) (digest.Digest, error) {
	if output.aux != nil {
		return getDigestFromAux(*output.aux)
	}

	if output.shortDigest != "" {
		data, _, err := d.dCli.ImageInspectWithRaw(ctx, output.shortDigest)
		if err != nil {
			return "", err
		}
		return digest.Digest(data.ID), nil
	}

	return "", fmt.Errorf("Could not find image digest in docker output")
}

func getDigestFromAux(aux json.RawMessage) (digest.Digest, error) {
	digestMap := make(map[string]string)
	err := json.Unmarshal(aux, &digestMap)
	if err != nil {
		return "", errors.Wrap(err, "getDigestFromAux")
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
		return "", fmt.Errorf("digest too short: %s", str)
	}
	return fmt.Sprintf("%s%s", ImageTagPrefix, str[:16]), nil
}

func digestMatchesRef(ref reference.NamedTagged, digest digest.Digest) bool {
	digestHash := digest.Encoded()
	tag := ref.Tag()
	if len(tag) <= len(ImageTagPrefix) {
		return false
	}

	tagHash := tag[len(ImageTagPrefix):]
	return strings.HasPrefix(digestHash, tagHash)
}

var oldDigestRegexp = regexp.MustCompile(`^Successfully built ([0-9a-f]+)\s*$`)

type dockerOutput struct {
	aux         *json.RawMessage
	shortDigest string
}

func indent(text, indent string) string {
	if text == "" {
		return indent + text
	}
	if text[len(text)-1:] == "\n" {
		result := ""
		for _, j := range strings.Split(text[:len(text)-1], "\n") {
			result += indent + j + "\n"
		}
		return result
	}
	result := ""
	for _, j := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		result += indent + j + "\n"
	}
	return result[:len(result)-1]
}

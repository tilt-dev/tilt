package build

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/opencontainers/go-digest"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/container"

	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockerfile"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type dockerImageBuilder struct {
	dCli docker.Client

	// A set of extra labels to attach to all builds
	// created by this image builder.
	//
	// By default, all builds are labeled with a build mode.
	extraLabels dockerfile.Labels
}

type DockerBuilder interface {
	BuildImage(ctx context.Context, ps *PipelineState, refs container.RefSet, db model.DockerBuild, filter model.PathMatcher) (container.TaggedRefs, error)
	DumpImageDeployRef(ctx context.Context, ref string) (reference.NamedTagged, error)
	PushImage(ctx context.Context, name reference.NamedTagged) error
	TagRefs(ctx context.Context, refs container.RefSet, dig digest.Digest) (container.TaggedRefs, error)
	ImageExists(ctx context.Context, ref reference.NamedTagged) (bool, error)
}

func DefaultDockerBuilder(b *dockerImageBuilder) DockerBuilder {
	return b
}

var _ DockerBuilder = &dockerImageBuilder{}

func NewDockerImageBuilder(dCli docker.Client, extraLabels dockerfile.Labels) *dockerImageBuilder {
	return &dockerImageBuilder{
		dCli:        dCli,
		extraLabels: extraLabels,
	}
}

func (d *dockerImageBuilder) BuildImage(ctx context.Context, ps *PipelineState, refs container.RefSet, db model.DockerBuild, filter model.PathMatcher) (container.TaggedRefs, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "dib-BuildImage")
	defer span.Finish()

	paths := []PathMapping{
		{
			LocalPath:     db.BuildPath,
			ContainerPath: "/",
		},
	}
	return d.buildFromDf(ctx, ps, db, paths, filter, refs)
}

func (d *dockerImageBuilder) DumpImageDeployRef(ctx context.Context, ref string) (reference.NamedTagged, error) {
	refParsed, err := container.ParseNamed(ref)
	if err != nil {
		return nil, errors.Wrap(err, "DumpImageDeployRef")
	}

	data, _, err := d.dCli.ImageInspectWithRaw(ctx, ref)
	if err != nil {
		return nil, errors.Wrap(err, "DumpImageDeployRef")
	}
	dig := digest.Digest(data.ID)

	tag, err := digestAsTag(dig)
	if err != nil {
		return nil, errors.Wrap(err, "DumpImageDeployRef")
	}

	tagged, err := reference.WithTag(refParsed, tag)
	if err != nil {
		return nil, errors.Wrap(err, "DumpImageDeployRef")
	}

	return tagged, nil
}

// Tag the digest with the given name and wm-tilt tag.
func (d *dockerImageBuilder) TagRefs(ctx context.Context, refs container.RefSet, dig digest.Digest) (container.TaggedRefs, error) {
	tag, err := digestAsTag(dig)
	if err != nil {
		return container.TaggedRefs{}, errors.Wrap(err, "TagImage")
	}

	tagged, err := refs.TagRefs(tag)
	if err != nil {
		return container.TaggedRefs{}, errors.Wrap(err, "TagImage")
	}

	// Docker client only needs to care about the localImage
	err = d.dCli.ImageTag(ctx, dig.String(), tagged.LocalRef.String())
	if err != nil {
		return container.TaggedRefs{}, errors.Wrap(err, "TagImage#ImageTag")
	}

	return tagged, nil
}

// Push the specified ref up to the docker registry specified in the name.
//
// TODO(nick) In the future, I would like us to be smarter about checking if the kubernetes cluster
// we're running in has access to the given registry. And if it doesn't, we should either emit an
// error, or push to a registry that kubernetes does have access to (e.g., a local registry).
func (d *dockerImageBuilder) PushImage(ctx context.Context, ref reference.NamedTagged) error {
	l := logger.Get(ctx)

	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-PushImage")
	defer span.Finish()

	imagePushResponse, err := d.dCli.ImagePush(ctx, ref)
	if err != nil {
		return errors.Wrap(err, "PushImage#ImagePush")
	}

	defer func() {
		err := imagePushResponse.Close()
		if err != nil {
			l.Infof("unable to close imagePushResponse: %s", err)
		}
	}()

	_, err = readDockerOutput(ctx, imagePushResponse)
	if err != nil {
		return errors.Wrapf(err, "pushing image %q", ref.Name())
	}

	return nil
}

func (d *dockerImageBuilder) ImageExists(ctx context.Context, ref reference.NamedTagged) (bool, error) {
	_, _, err := d.dCli.ImageInspectWithRaw(ctx, ref.String())
	if err != nil {
		if client.IsErrNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "error checking if %s exists", ref.String())
	}
	return true, nil
}

func (d *dockerImageBuilder) buildFromDf(ctx context.Context, ps *PipelineState, db model.DockerBuild, paths []PathMapping, filter model.PathMatcher, refs container.RefSet) (container.TaggedRefs, error) {
	logger.Get(ctx).Infof("Building Dockerfile:\n%s\n", indent(db.Dockerfile, "  "))
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-buildFromDf")
	defer span.Finish()

	ps.StartBuildStep(ctx, "Tarring context…")

	// NOTE(maia): some people want to know what files we're adding (b/c `ADD . /` isn't descriptive)
	if logger.Get(ctx).Level().ShouldDisplay(logger.VerboseLvl) {
		for _, pm := range paths {
			ps.Printf(ctx, pm.PrettyStr())
		}
	}

	pr, pw := io.Pipe()
	go func(ctx context.Context) {
		err := tarContextAndUpdateDf(ctx, pw, dockerfile.Dockerfile(db.Dockerfile), paths, filter)
		if err != nil {
			_ = pw.CloseWithError(err)
		} else {
			_ = pw.Close()
		}
	}(ctx)

	defer func() {
		_ = pr.Close()
	}()

	ps.StartBuildStep(ctx, "Building image")
	spanBuild, ctx := opentracing.StartSpanFromContext(ctx, "daemon-ImageBuild")
	imageBuildResponse, err := d.dCli.ImageBuild(
		ctx,
		pr,
		Options(pr, db),
	)
	spanBuild.Finish()
	if err != nil {
		return container.TaggedRefs{}, err
	}

	defer func() {
		err := imageBuildResponse.Body.Close()
		if err != nil {
			logger.Get(ctx).Infof("unable to close imagePushResponse: %s", err)
		}
	}()

	digest, err := d.getDigestFromBuildOutput(ps.AttachLogger(ctx), imageBuildResponse.Body)
	if err != nil {
		return container.TaggedRefs{}, err
	}

	tagged, err := d.TagRefs(ctx, refs, digest)
	if err != nil {
		return container.TaggedRefs{}, errors.Wrap(err, "PushImage")
	}

	return tagged, nil
}

func (d *dockerImageBuilder) getDigestFromBuildOutput(ctx context.Context, reader io.Reader) (digest.Digest, error) {
	result, err := readDockerOutput(ctx, reader)
	if err != nil {
		return "", errors.Wrap(err, "ImageBuild")
	}

	digest, err := d.getDigestFromDockerOutput(ctx, result)
	if err != nil {
		return "", errors.Wrap(err, "getDigestFromBuildOutput")
	}

	return digest, nil
}

var dockerBuildCleanupRexes = []*regexp.Regexp{
	// the "runc did not determinate sucessfully" just seems redundant on top of "executor failed running"
	// nolint
	regexp.MustCompile("(executor failed running.*): runc did not terminate sucessfully"), // sucessfully (sic)
	// when a file is missing, it generates an error like "failed to compute cache key: foo.txt not found: not found"
	// most of that seems redundant and/or confusing
	regexp.MustCompile("failed to compute cache key: (.* not found): not found"),
}

// buildkit emits errors that might be useful for people who are into buildkit internals, but aren't really
// at the optimal level for people who just wanna build something
// ideally we'll get buildkit to emit errors with more structure so that we don't have to rely on string manipulation,
// but to have impact via that route, we've got to get the change in and users have to upgrade to a version of docker
// that has that change. So let's clean errors up here until that's in a good place.
func cleanupDockerBuildError(err string) string {
	// this is pretty much always the same, and meaningless noise to most users
	ret := strings.TrimPrefix(err, "failed to solve with frontend dockerfile.v0: ")
	ret = strings.TrimPrefix(ret, "failed to solve with frontend gateway.v0: ")
	ret = strings.TrimPrefix(ret, "rpc error: code = Unknown desc = ")
	ret = strings.TrimPrefix(ret, "failed to build LLB: ")
	for _, re := range dockerBuildCleanupRexes {
		ret = re.ReplaceAllString(ret, "$1")
	}
	return ret
}

type dockerMessageID string

// Docker API commands stream back a sequence of JSON messages.
//
// The result of the command is in a JSON object with field "aux".
//
// Errors are reported in a JSON object with field "errorDetail"
//
// NOTE(nick): I haven't found a good document describing this protocol
// but you can find it implemented in Docker here:
// https://github.com/moby/moby/blob/1da7d2eebf0a7a60ce585f89a05cebf7f631019c/pkg/jsonmessage/jsonmessage.go#L139
func readDockerOutput(ctx context.Context, reader io.Reader) (dockerOutput, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-readDockerOutput")
	defer span.Finish()

	progressLastPrinted := make(map[dockerMessageID]time.Time)

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

			logger.Get(ctx).Write(logger.InfoLvl, []byte(msg))
			if strings.HasPrefix(msg, "Run") || strings.HasPrefix(msg, "Running") {
				innerSpan, ctx = opentracing.StartSpanFromContext(ctx, msg)
			}
		}

		if message.ErrorMessage != "" {
			return dockerOutput{}, errors.New(cleanupDockerBuildError(message.ErrorMessage))
		}

		if message.Error != nil {
			return dockerOutput{}, errors.New(cleanupDockerBuildError(message.Error.Message))
		}

		id := dockerMessageID(message.ID)
		if id != "" && message.Progress != nil {
			// Add a small 2-second backoff so that we don't overwhelm the logstore.
			lastPrinted, hasBeenPrinted := progressLastPrinted[id]
			shouldPrint := !hasBeenPrinted ||
				message.Progress.Current == message.Progress.Total ||
				time.Since(lastPrinted) > 2*time.Second
			shouldSkip := message.Progress.Current == 0 &&
				(message.Status == "Waiting" || message.Status == "Preparing")
			if shouldPrint && !shouldSkip {
				fields := logger.Fields{logger.FieldNameProgressID: message.ID}
				if message.Progress.Current == message.Progress.Total {
					fields[logger.FieldNameProgressMustPrint] = "1"
				}
				logger.Get(ctx).WithFields(fields).
					Infof("%s: %s %s", id, message.Status, message.Progress.String())
				progressLastPrinted[id] = time.Now()
			}
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
	return b.parseAndPrint(toVertexes(resp))
}

func toVertexes(resp controlapi.StatusResponse) ([]*vertex, []*vertexLog, []*vertexStatus) {
	vertexes := []*vertex{}
	logs := []*vertexLog{}
	statuses := []*vertexStatus{}

	for _, v := range resp.Vertexes {
		duration := time.Duration(0)
		started := v.Started != nil
		completed := v.Completed != nil
		if started && completed {
			duration = (*v.Completed).Sub((*v.Started))
		}
		vertexes = append(vertexes, &vertex{
			digest:    v.Digest,
			name:      v.Name,
			error:     v.Error,
			started:   started,
			completed: completed,
			cached:    v.Cached,
			duration:  duration,
		})

	}
	for _, v := range resp.Logs {
		logs = append(logs, &vertexLog{
			vertex: v.Vertex,
			msg:    v.Msg,
		})
	}
	for _, s := range resp.Statuses {
		statuses = append(statuses, &vertexStatus{
			vertex:    s.Vertex,
			id:        s.ID,
			total:     s.Total,
			current:   s.Current,
			timestamp: s.Timestamp,
		})
	}
	return vertexes, logs, statuses
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

	return "", fmt.Errorf("Docker is not responding. Maybe Docker is out of disk space? Try running `docker system prune`")
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

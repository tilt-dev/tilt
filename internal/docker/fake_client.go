package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/model"
)

const ExampleBuildSHA1 = "sha256:11cd0b38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab"

const ExampleBuildOutput1 = `{"stream":"Step 1/1 : FROM alpine"}
	{"stream":"\n"}
	{"stream":" ---\u003e 11cd0b38bc3c\n"}
	{"aux":{"ID":"sha256:11cd0b38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab"}}
	{"stream":"Successfully built 11cd0b38bc3c\n"}
	{"stream":"Successfully tagged hi:latest\n"}
`
const ExampleBuildOutputV1_23 = `{"stream":"Step 1/1 : FROM alpine"}
	{"stream":"\n"}
	{"stream":" ---\u003e 11cd0b38bc3c\n"}
	{"stream":"Successfully built 11cd0b38bc3c\n"}
	{"stream":"Successfully tagged hi:latest\n"}
`

const ExamplePushSHA1 = "sha256:cc5f4c463f81c55183d8d737ba2f0d30b3e6f3670dbe2da68f0aac168e93fbb1"

var ExamplePushOutput1 = `{"status":"The push refers to repository [localhost:5005/myimage]"}
	{"status":"Preparing","progressDetail":{},"id":"2a88b569da78"}
	{"status":"Preparing","progressDetail":{},"id":"73046094a9b8"}
	{"status":"Pushing","progressDetail":{"current":512,"total":41},"progress":"[==================================================\u003e]     512B","id":"2a88b569da78"}
	{"status":"Pushing","progressDetail":{"current":68608,"total":4413370},"progress":"[\u003e                                                  ]  68.61kB/4.413MB","id":"73046094a9b8"}
	{"status":"Pushing","progressDetail":{"current":5120,"total":41},"progress":"[==================================================\u003e]   5.12kB","id":"2a88b569da78"}
	{"status":"Pushed","progressDetail":{},"id":"2a88b569da78"}
	{"status":"Pushing","progressDetail":{"current":1547776,"total":4413370},"progress":"[=================\u003e                                 ]  1.548MB/4.413MB","id":"73046094a9b8"}
	{"status":"Pushing","progressDetail":{"current":3247616,"total":4413370},"progress":"[====================================\u003e              ]  3.248MB/4.413MB","id":"73046094a9b8"}
	{"status":"Pushing","progressDetail":{"current":4672000,"total":4413370},"progress":"[==================================================\u003e]  4.672MB","id":"73046094a9b8"}
	{"status":"Pushed","progressDetail":{},"id":"73046094a9b8"}
	{"status":"tilt-11cd0b38bc3ceb95: digest: sha256:cc5f4c463f81c55183d8d737ba2f0d30b3e6f3670dbe2da68f0aac168e93fbb1 size: 735"}
	{"progressDetail":{},"aux":{"Tag":"tilt-11cd0b38bc3ceb95","Digest":"sha256:cc5f4c463f81c55183d8d737ba2f0d30b3e6f3670dbe2da68f0aac168e93fbb1","Size":735}}`

const (
	TestPod       = "test_pod"
	TestContainer = "test_container"
)

var DefaultContainerListOutput = map[string][]types.Container{
	TestPod: []types.Container{
		types.Container{ID: TestContainer, ImageID: ExampleBuildSHA1, Command: "./stuff"},
	},
	"two-containers": []types.Container{
		types.Container{ID: "not a match", ImageID: ExamplePushSHA1, Command: "/pause"},
		types.Container{ID: "the right container", ImageID: ExampleBuildSHA1, Command: "./stuff"},
	},
}

type ExecCall struct {
	Container string
	Cmd       model.Cmd
}

type FakeClient struct {
	PushCount   int
	PushImage   string
	PushOptions types.ImagePushOptions
	PushOutput  string

	BuildCount   int
	BuildOptions BuildOptions
	BuildOutput  string

	TagCount  int
	TagSource string
	TagTarget string

	ContainerListOutput map[string][]types.Container

	CopyCount     int
	CopyContainer string
	CopyContent   io.Reader

	ExecCalls        []ExecCall
	ExecErrorToThrow error // next call to Exec will throw this err (after which we clear the error)

	RestartsByContainer map[string]int
	RemovedImageIDs     []string

	Images map[string]types.ImageInspect
}

func NewFakeClient() *FakeClient {
	return &FakeClient{
		PushOutput:          ExamplePushOutput1,
		BuildOutput:         ExampleBuildOutput1,
		ContainerListOutput: make(map[string][]types.Container),
		RestartsByContainer: make(map[string]int),
		Images:              make(map[string]types.ImageInspect),
	}
}

func (c *FakeClient) SetContainerListOutput(output map[string][]types.Container) {
	c.ContainerListOutput = output
}

func (c *FakeClient) SetDefaultContainerListOutput() {
	c.SetContainerListOutput(DefaultContainerListOutput)
}

func (c *FakeClient) ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error) {
	nameFilter := options.Filters.Get("name")
	if len(nameFilter) != 1 {
		return nil, fmt.Errorf("expected one filter for 'name', got: %v", nameFilter)
	}

	if len(c.ContainerListOutput) == 0 {
		return nil, fmt.Errorf("FakeClient ContainerListOutput not set (use `SetContainerListOutput`)")
	}
	res := c.ContainerListOutput[nameFilter[0]]

	// unset containerListOutput
	c.ContainerListOutput = nil

	return res, nil
}

func (c *FakeClient) ContainerRestartNoWait(ctx context.Context, containerID string) error {
	c.RestartsByContainer[containerID]++
	return nil
}

func (c *FakeClient) ExecInContainer(ctx context.Context, cID container.ID, cmd model.Cmd, out io.Writer) error {
	execCall := ExecCall{
		Container: cID.String(),
		Cmd:       cmd,
	}
	c.ExecCalls = append(c.ExecCalls, execCall)

	// If we're supposed to throw an error on this call, throw it (and reset ErrorToThrow)
	err := c.ExecErrorToThrow
	c.ExecErrorToThrow = nil
	return err
}

func (c *FakeClient) CopyToContainerRoot(ctx context.Context, container string, content io.Reader) error {
	c.CopyCount++
	c.CopyContainer = container
	c.CopyContent = content
	return nil
}

func (c *FakeClient) ImagePush(ctx context.Context, image string, options types.ImagePushOptions) (io.ReadCloser, error) {
	c.PushCount++
	c.PushImage = image
	c.PushOptions = options
	return NewFakeDockerResponse(c.PushOutput), nil
}

func (c *FakeClient) ImageBuild(ctx context.Context, buildContext io.Reader, options BuildOptions) (types.ImageBuildResponse, error) {
	c.BuildCount++
	c.BuildOptions = options
	return types.ImageBuildResponse{Body: NewFakeDockerResponse(c.BuildOutput)}, nil
}

func (c *FakeClient) ImageTag(ctx context.Context, source, target string) error {
	c.TagCount++
	c.TagSource = source
	c.TagTarget = target
	return nil
}

func (c *FakeClient) ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error) {
	result, ok := c.Images[imageID]
	if ok {
		return result, nil, nil
	}
	return types.ImageInspect{}, nil, notFoundError{}
}

func (c *FakeClient) ImageList(ctx context.Context, options types.ImageListOptions) ([]types.ImageSummary, error) {
	summaries := make([]types.ImageSummary, c.BuildCount)
	for i := range summaries {
		summaries[i] = types.ImageSummary{
			ID:      fmt.Sprintf("build-id-%d", i),
			Created: time.Now().Add(-time.Second).Unix(),
		}
	}
	return summaries, nil
}

func (c *FakeClient) ImageRemove(ctx context.Context, imageID string, options types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error) {
	c.RemovedImageIDs = append(c.RemovedImageIDs, imageID)
	sort.Strings(c.RemovedImageIDs)
	return nil, nil
}

var _ Client = &FakeClient{}

type fakeDockerResponse struct {
	*bytes.Buffer
}

func NewFakeDockerResponse(contents string) fakeDockerResponse {
	return fakeDockerResponse{Buffer: bytes.NewBufferString(contents)}
}

func (r fakeDockerResponse) Close() error { return nil }

var _ io.ReadCloser = fakeDockerResponse{}

type notFoundError struct {
}

func (e notFoundError) NotFound() bool {
	return true
}

func (e notFoundError) Error() string {
	return "fake docker client error: object not found"
}

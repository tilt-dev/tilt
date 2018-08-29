package build

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
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

const ExmaplePushSHA1 = "sha256:cc5f4c463f81c55183d8d737ba2f0d30b3e6f3670dbe2da68f0aac168e93fbb1"

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
	testPod       = "test_pod"
	testContainer = "test_container"
)

var ContainersListByName = map[string][]types.Container{
	testPod: []types.Container{
		types.Container{ID: testContainer, Command: "./stuff"},
	},
	"one-pause-cmd": []types.Container{
		types.Container{ID: "not a match", Command: pauseCmd},
		types.Container{ID: "the right container", Command: "./stuff"},
	},
	"too-many": []types.Container{
		types.Container{ID: "nope", Command: "./stuff"},
		types.Container{ID: "nah", Command: "./things"},
		types.Container{ID: "nuh-uh", Command: "./nonsense"},
	},
	"all-pause": []types.Container{
		types.Container{ID: "pause container", Command: pauseCmd},
		types.Container{ID: "also pause", Command: pauseCmd},
	},
}

type ExecCall struct {
	Container string
	Cmd       model.Cmd
}

type FakeDockerClient struct {
	PushCount   int
	PushImage   string
	PushOptions types.ImagePushOptions
	PushOutput  io.ReadCloser

	BuildCount   int
	BuildOptions types.ImageBuildOptions
	BuildOutput  io.ReadCloser

	TagCount  int
	TagSource string
	TagTarget string

	ContainerListOutput map[string][]types.Container

	CopyCount     int
	CopyContainer string
	CopyPath      string
	CopyContent   io.Reader
	CopyOptions   types.CopyToContainerOptions

	ExecCalls []ExecCall

	RestartsByContainer map[string]int
}

func NewFakeDockerClient() *FakeDockerClient {
	return &FakeDockerClient{
		PushOutput:          NewFakeDockerResponse(ExamplePushOutput1),
		BuildOutput:         NewFakeDockerResponse(ExampleBuildOutput1),
		ContainerListOutput: ContainersListByName,
		RestartsByContainer: make(map[string]int),
	}
}

func (c *FakeDockerClient) ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error) {
	nameFilter := options.Filters.Get("name")
	if len(nameFilter) != 1 {
		return nil, fmt.Errorf("expected one filter for 'name', got: %v", nameFilter)
	}

	return c.ContainerListOutput[nameFilter[0]], nil
}

func (c *FakeDockerClient) ContainerRestart(ctx context.Context, containerID string, timeout *time.Duration) error {
	c.RestartsByContainer[containerID]++
	return nil
}

func (c *FakeDockerClient) ExecInContainer(ctx context.Context, cID containerID, cmd model.Cmd) error {
	execCall := ExecCall{
		Container: cID.String(),
		Cmd:       cmd,
	}
	c.ExecCalls = append(c.ExecCalls, execCall)

	return nil
}

func (c *FakeDockerClient) CopyToContainer(ctx context.Context, container, path string, content io.Reader, options types.CopyToContainerOptions) error {
	c.CopyCount++
	c.CopyContainer = container
	c.CopyPath = path
	c.CopyContent = content
	c.CopyOptions = options
	return nil
}

func (c *FakeDockerClient) ImagePush(ctx context.Context, image string, options types.ImagePushOptions) (io.ReadCloser, error) {
	c.PushCount++
	c.PushImage = image
	c.PushOptions = options
	return c.PushOutput, nil
}

func (c *FakeDockerClient) ImageBuild(ctx context.Context, buildContext io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error) {
	c.BuildCount++
	c.BuildOptions = options
	return types.ImageBuildResponse{Body: c.BuildOutput}, nil
}

func (c *FakeDockerClient) ImageTag(ctx context.Context, source, target string) error {
	c.TagCount++
	c.TagSource = source
	c.TagTarget = target
	return nil
}

var _ DockerClient = &FakeDockerClient{}

type fakeDockerResponse struct {
	*bytes.Buffer
}

func NewFakeDockerResponse(contents string) fakeDockerResponse {
	return fakeDockerResponse{Buffer: bytes.NewBufferString(contents)}
}

func (r fakeDockerResponse) Close() error { return nil }

var _ io.ReadCloser = fakeDockerResponse{}

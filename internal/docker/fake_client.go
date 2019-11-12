package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/docker/go-units"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/pkg/model"
)

const ExampleBuildSHA1 = "sha256:11cd0b38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab"

const ExampleBuildOutput1 = `{"stream":"Run 1/1 : FROM alpine"}
	{"stream":"\n"}
	{"stream":" ---\u003e 11cd0b38bc3c\n"}
	{"aux":{"ID":"sha256:11cd0b38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab"}}
	{"stream":"Successfully built 11cd0b38bc3c\n"}
	{"stream":"Successfully tagged hi:latest\n"}
`
const ExampleBuildOutputV1_23 = `{"stream":"Run 1/1 : FROM alpine"}
	{"stream":"\n"}
	{"stream":" ---\u003e 11cd0b38bc3c\n"}
	{"stream":"Successfully built 11cd0b38bc3c\n"}
	{"stream":"Successfully tagged hi:latest\n"}
`

// same as ExampleBuildOutput1 but with a different digest
const ExampleBuildOutput2 = `{"stream":"Run 1/1 : FROM alpine"}
	{"stream":"\n"}
	{"stream":" ---\u003e 20372c132963\n"}
	{"aux":{"ID":"sha256:20372c132963eb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af20372c132963"}}
	{"stream":"Successfully built 20372c132963\n"}
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

	BuildCount        int
	BuildOptions      BuildOptions
	BuildOutput       string
	BuildErrorToThrow error // next call to Build will throw this err (after which we clear the error)

	ImageListCount int
	ImageListOpts  []types.ImageListOptions

	TagCount  int
	TagSource string
	TagTarget string

	ContainerListOutput map[string][]types.Container

	CopyCount     int
	CopyContainer string
	CopyContent   io.Reader

	ExecCalls         []ExecCall
	ExecErrorsToThrow []error // next call to exec will throw ExecError[0] (which we then pop)

	RestartsByContainer map[string]int
	RemovedImageIDs     []string

	Images            map[string]types.ImageInspect
	Orchestrator      model.Orchestrator
	CheckConnectedErr error

	ThrowNewVersionError   bool
	BuildCachePruneErr     error
	BuildCachePruneOpts    types.BuildCachePruneOptions
	BuildCachesPruned      []string
	ContainersPruneErr     error
	ContainersPruneFilters filters.Args
	ContainersPruned       []string
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

func (c *FakeClient) SetOrchestrator(orc model.Orchestrator) {
	c.Orchestrator = orc
}
func (c *FakeClient) CheckConnected() error {
	return c.CheckConnectedErr
}
func (c *FakeClient) Env() Env {
	return Env{}
}
func (c *FakeClient) BuilderVersion() types.BuilderVersion {
	return types.BuilderV1
}
func (c *FakeClient) ServerVersion() types.Version {
	return types.Version{}
}

func (c *FakeClient) SetExecError(err error) {
	c.ExecErrorsToThrow = []error{err}
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

	// If we're supposed to throw an error on this call, throw it (and pop from
	// the list of ErrorsToThrow)
	var err error
	if len(c.ExecErrorsToThrow) > 0 {
		err = c.ExecErrorsToThrow[0]
		c.ExecErrorsToThrow = append([]error{}, c.ExecErrorsToThrow[1:]...)
	}

	return err
}

func (c *FakeClient) CopyToContainerRoot(ctx context.Context, container string, content io.Reader) error {
	c.CopyCount++
	c.CopyContainer = container
	c.CopyContent = content
	return nil
}

func (c *FakeClient) ImagePush(ctx context.Context, ref reference.NamedTagged) (io.ReadCloser, error) {
	c.PushCount++
	c.PushImage = ref.String()
	return NewFakeDockerResponse(c.PushOutput), nil
}

func (c *FakeClient) ImageBuild(ctx context.Context, buildContext io.Reader, options BuildOptions) (types.ImageBuildResponse, error) {
	c.BuildCount++
	c.BuildOptions = options

	// If we're supposed to throw an error on this call, throw it (and reset ErrorToThrow)
	err := c.BuildErrorToThrow
	if err != nil {
		c.BuildErrorToThrow = nil
		return types.ImageBuildResponse{}, err
	}

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
	return types.ImageInspect{}, nil, newNotFoundErrorf("fakeClient.Images key: %s", imageID)
}

func (c *FakeClient) ImageList(ctx context.Context, options types.ImageListOptions) ([]types.ImageSummary, error) {
	c.ImageListOpts = append(c.ImageListOpts, options)
	summaries := make([]types.ImageSummary, c.ImageListCount)
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
	return []types.ImageDeleteResponseItem{
		types.ImageDeleteResponseItem{Deleted: imageID},
	}, nil
}

func (c *FakeClient) NewVersionError(APIrequired, feature string) error {
	if c.ThrowNewVersionError {
		c.ThrowNewVersionError = false
		return c.VersionError(APIrequired, feature)
	}
	return nil
}

func (c *FakeClient) VersionError(APIrequired, feature string) error {
	return fmt.Errorf("%q requires API version %s, but the Docker daemon API version is... something else", feature, APIrequired)
}

func (c *FakeClient) BuildCachePrune(ctx context.Context, opts types.BuildCachePruneOptions) (*types.BuildCachePruneReport, error) {
	if err := c.BuildCachePruneErr; err != nil {
		c.BuildCachePruneErr = nil
		return nil, err
	}

	c.BuildCachePruneOpts = types.BuildCachePruneOptions{
		All:         opts.All,
		KeepStorage: opts.KeepStorage,
		Filters:     opts.Filters.Clone(),
	}
	report := &types.BuildCachePruneReport{
		CachesDeleted:  c.BuildCachesPruned,
		SpaceReclaimed: uint64(units.MB * len(c.BuildCachesPruned)), // 1MB per cache pruned
	}
	c.BuildCachesPruned = nil
	return report, nil
}

func (c *FakeClient) ContainersPrune(ctx context.Context, pruneFilters filters.Args) (types.ContainersPruneReport, error) {
	if err := c.ContainersPruneErr; err != nil {
		c.ContainersPruneErr = nil
		return types.ContainersPruneReport{}, err
	}

	c.ContainersPruneFilters = pruneFilters.Clone()
	report := types.ContainersPruneReport{
		ContainersDeleted: c.ContainersPruned,
		SpaceReclaimed:    uint64(units.MB * len(c.ContainersPruned)), // 1MB per container pruned
	}
	c.ContainersPruned = nil
	return report, nil
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
	details string
}

func newNotFoundErrorf(s string, a ...interface{}) notFoundError {
	return notFoundError{details: fmt.Sprintf(s, a...)}
}

func (e notFoundError) NotFound() bool {
	return true
}

func (e notFoundError) Error() string {
	return fmt.Sprintf("fake docker client error: object not found (%s)", e.details)
}

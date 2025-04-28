package dockercompose

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"testing"

	"github.com/compose-spec/compose-go/v2/loader"

	"github.com/compose-spec/compose-go/v2/types"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type FakeDCClient struct {
	t   *testing.T
	ctx context.Context

	mu sync.Mutex

	ContainerIDDefault   container.ID
	ContainerIDByService map[string]container.ID
	eventJson            chan string
	ConfigOutput         string
	VersionOutput        string

	upCalls   []UpCall
	downCalls []DownCall
	rmCalls   []RmCall
	DownError error
	RmError   error
	RmOutput  string
	WorkDir   string
}

var _ DockerComposeClient = &FakeDCClient{}

// Represents a single call to Up
type UpCall struct {
	Spec        v1alpha1.DockerComposeServiceSpec
	ShouldBuild bool
}

// Represents a single call to Down
type DownCall struct {
	Proj v1alpha1.DockerComposeProject
}

type RmCall struct {
	Specs []v1alpha1.DockerComposeServiceSpec
}

func NewFakeDockerComposeClient(t *testing.T, ctx context.Context) *FakeDCClient {
	return &FakeDCClient{
		t:                    t,
		ctx:                  ctx,
		eventJson:            make(chan string, 100),
		ContainerIDByService: make(map[string]container.ID),
	}
}

func (c *FakeDCClient) Up(ctx context.Context, spec v1alpha1.DockerComposeServiceSpec,
	shouldBuild bool, stdout, stderr io.Writer) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.upCalls = append(c.upCalls, UpCall{spec, shouldBuild})
	return nil
}

func (c *FakeDCClient) Down(ctx context.Context, proj v1alpha1.DockerComposeProject, stdout, stderr io.Writer) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.downCalls = append(c.downCalls, DownCall{proj})
	if c.DownError != nil {
		err := c.DownError
		c.DownError = nil
		return err
	}
	return nil
}

func (c *FakeDCClient) Rm(ctx context.Context, specs []v1alpha1.DockerComposeServiceSpec, stdout, stderr io.Writer) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.rmCalls = append(c.rmCalls, RmCall{specs})
	if c.RmError != nil {
		err := c.RmError
		c.RmError = nil
		return err
	}

	_, _ = fmt.Fprint(stdout, c.RmOutput)
	return nil
}

func (c *FakeDCClient) StreamEvents(ctx context.Context, p v1alpha1.DockerComposeProject) (<-chan string, error) {
	events := make(chan string, 10)
	go func() {
		for {
			select {
			case event := <-c.eventJson:
				select {
				case events <- event: // send event to channel (unless it's full)
				default:
					panic(fmt.Sprintf("no room on events channel to send event: '%s'. Something "+
						"is wrong (or you need to increase the buffer).", event))
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return events, nil
}

func (c *FakeDCClient) SendEvent(evt Event) error {
	j, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	c.eventJson <- string(j)
	return nil
}

func (c *FakeDCClient) Config(_ context.Context, _ []string) (string, error) {
	return c.ConfigOutput, nil
}

func (c *FakeDCClient) Project(ctx context.Context, m v1alpha1.DockerComposeProject) (*types.Project, error) {
	// this is a dummy ProjectOptions that lets us use compose's logic to apply options
	// for consistency, but we have to then pull the data out ourselves since we're calling
	// loader.Load ourselves
	opts, err := composeProjectOptions(m, nil)
	if err != nil {
		return nil, err
	}

	workDir := opts.WorkingDir
	projectName := opts.Name
	if projectName == "" {
		projectName = loader.NormalizeProjectName(workDir)
	}
	if projectName == "" {
		projectName = "fakedc"
	}

	p, err := loader.LoadWithContext(ctx, types.ConfigDetails{
		WorkingDir: workDir,
		ConfigFiles: []types.ConfigFile{
			{
				Content: []byte(c.ConfigOutput),
			},
		},
		Environment: opts.Environment,
	}, dcLoaderOption(projectName))
	return p, err
}

func (c *FakeDCClient) ContainerID(ctx context.Context, spec v1alpha1.DockerComposeServiceSpec) (container.ID, error) {
	id, ok := c.ContainerIDByService[spec.Service]
	if ok {
		return id, nil
	}
	return c.ContainerIDDefault, nil
}

func (c *FakeDCClient) Version(_ context.Context) (string, string, error) {
	if c.VersionOutput != "" {
		return c.VersionOutput, "tilt-fake", nil
	}
	// default to a "known good" version that won't produce warnings
	return "v1.29.2", "tilt-fake", nil
}

func (c *FakeDCClient) UpCalls() []UpCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]UpCall{}, c.upCalls...)
}

func (c *FakeDCClient) DownCalls() []DownCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]DownCall{}, c.downCalls...)
}

func (c *FakeDCClient) RmCalls() []RmCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]RmCall{}, c.rmCalls...)
}

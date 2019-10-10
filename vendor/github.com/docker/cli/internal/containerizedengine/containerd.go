package containerizedengine

import (
	"context"
	"io"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/remotes/docker"
	clitypes "github.com/docker/cli/types"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/jsonmessage"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// NewClient returns a new containerizedengine client
// This client can be used to manage the lifecycle of
// dockerd running as a container on containerd.
func NewClient(sockPath string) (clitypes.ContainerizedClient, error) {
	if sockPath == "" {
		sockPath = containerdSockPath
	}
	cclient, err := containerd.New(sockPath)
	if err != nil {
		return nil, err
	}
	return &baseClient{
		cclient: cclient,
	}, nil
}

// Close will close the underlying clients
func (c *baseClient) Close() error {
	return c.cclient.Close()
}

func (c *baseClient) pullWithAuth(ctx context.Context, imageName string, out clitypes.OutStream,
	authConfig *types.AuthConfig) (containerd.Image, error) {

	resolver := docker.NewResolver(docker.ResolverOptions{
		Credentials: func(string) (string, string, error) {
			return authConfig.Username, authConfig.Password, nil
		},
	})

	ongoing := newJobs(imageName)
	pctx, stopProgress := context.WithCancel(ctx)
	progress := make(chan struct{})
	bufin, bufout := io.Pipe()

	go func() {
		showProgress(pctx, ongoing, c.cclient.ContentStore(), bufout)
	}()

	go func() {
		jsonmessage.DisplayJSONMessagesToStream(bufin, out, nil)
		close(progress)
	}()

	h := images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		if desc.MediaType != images.MediaTypeDockerSchema1Manifest {
			ongoing.add(desc)
		}
		return nil, nil
	})

	image, err := c.cclient.Pull(ctx, imageName,
		containerd.WithResolver(resolver),
		containerd.WithImageHandler(h),
		containerd.WithPullUnpack)
	stopProgress()

	if err != nil {
		return nil, err
	}
	<-progress
	return image, nil
}

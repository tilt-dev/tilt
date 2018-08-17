package build

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	digest "github.com/opencontainers/go-digest"
	"github.com/windmilleng/tilt/internal/model"
)

var sleepForeverCmd = model.Cmd{Argv: []string{"tail", "-f", "/dev/null"}}

type containerPool struct {
	docker  *client.Client
	claimed map[containerID]digest.Digest
	mu      *sync.Mutex
}

func newContainerPool(docker *client.Client) containerPool {
	return containerPool{
		docker:  docker,
		claimed: make(map[containerID]digest.Digest, 0),
		mu:      &sync.Mutex{},
	}
}

// Claim a container, creating a new one if necessary.
//
// Once the container is claimed, no one is allowed to use it until it's released.
//
// TODO(nick): Right now, this always creates a new container.
func (p containerPool) claimContainer(ctx context.Context, digest digest.Digest) (containerID, error) {
	config := containerConfigRunCmd(digest, sleepForeverCmd)
	resp, err := p.docker.ContainerCreate(ctx, config, nil, nil, "")
	if err != nil {
		return "", err
	}

	cID := resp.ID

	err = p.docker.ContainerStart(ctx, cID, types.ContainerStartOptions{})
	if err != nil {
		return "", err
	}
	containerID := containerID(cID)
	p.mu.Lock()
	p.claimed[containerID] = digest
	p.mu.Unlock()
	return containerID, nil
}

// Snapshot a container and release it back into the pool, for
// use in future builds.
//
// TODO(nick): Right now, this deletes the container immediately. We'll eventually
// add persistence of running containers.
func (p containerPool) commitContainer(ctx context.Context, cID containerID, entrypoint model.Cmd) (digest.Digest, error) {
	p.mu.Lock()
	baseDigest, ok := p.claimed[cID]
	delete(p.claimed, cID)
	p.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("container %s unknown to pool", baseDigest)
	}

	opts := types.ContainerCommitOptions{}
	if !entrypoint.Empty() {
		// Attach an entrypoint provided by the caller.
		opts.Changes = []string{entrypoint.EntrypointStr()}
	} else {
		// When the container was claimed, we overwrote the entrypoint with tail -f /dev/null.
		// Let's restore it from the original digest.
		inspected, _, err := p.docker.ImageInspectWithRaw(ctx, string(baseDigest))
		if err != nil {
			return "", fmt.Errorf("error inspected image: %s", baseDigest)
		}

		if inspected.Config != nil && len(inspected.Config.Entrypoint) > 0 {
			cmd := model.Cmd{Argv: inspected.Config.Entrypoint}
			opts.Changes = []string{cmd.EntrypointStr()}
		} else if inspected.Config != nil && len(inspected.Config.Cmd) > 0 {
			cmd := model.Cmd{Argv: inspected.Config.Cmd}
			opts.Changes = []string{cmd.EntrypointStr()}
		} else {
			cmd := model.Cmd{Argv: []string{"sh", "-c", "# NOTE(nick): no cmd found"}}
			opts.Changes = []string{cmd.EntrypointStr()}
		}
	}

	commit, err := p.docker.ContainerCommit(ctx, string(cID), opts)
	if err != nil {
		return "", fmt.Errorf("releaseContainer#Commit: %v", err)
	}

	go func() {
		err = p.docker.ContainerKill(ctx, string(cID), "SIGKILL")
		if err != nil {
			log.Printf("failed to kill container: %v\n", err)
			return
		}
		err = p.docker.ContainerRemove(ctx, string(cID), types.ContainerRemoveOptions{})
		if err != nil {
			log.Printf("failed to remove container: %v\n", err)
			return
		}
	}()
	return digest.Digest(commit.ID), nil
}

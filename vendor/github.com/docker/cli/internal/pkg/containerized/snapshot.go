package containerized

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/diff/apply"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/rootfs"
	"github.com/containerd/containerd/snapshots"
	"github.com/opencontainers/image-spec/identity"
)

const (
	gcRoot           = "containerd.io/gc.root"
	timestampFormat  = "01-02-2006-15:04:05"
	previousRevision = "docker.com/revision.previous"
	imageLabel       = "docker.com/revision.image"
)

// ErrNoPreviousRevision returned if the container has to previous revision
var ErrNoPreviousRevision = errors.New("no previous revision")

// WithNewSnapshot creates a new snapshot managed by containerized
func WithNewSnapshot(i containerd.Image) containerd.NewContainerOpts {
	return func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
		if c.Snapshotter == "" {
			c.Snapshotter = containerd.DefaultSnapshotter
		}
		r, err := create(ctx, client, i, c.ID, "")
		if err != nil {
			return err
		}
		c.SnapshotKey = r.Key
		c.Image = i.Name()
		return nil
	}
}

// WithUpgrade upgrades an existing container's image to a new one
func WithUpgrade(i containerd.Image) containerd.UpdateContainerOpts {
	return func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
		revision, err := save(ctx, client, i, c)
		if err != nil {
			return err
		}
		c.Image = i.Name()
		c.SnapshotKey = revision.Key
		return nil
	}
}

// WithRollback rolls back to the previous container's revision
func WithRollback(ctx context.Context, client *containerd.Client, c *containers.Container) error {
	prev, err := previous(ctx, client, c)
	if err != nil {
		return err
	}
	ss := client.SnapshotService(c.Snapshotter)
	sInfo, err := ss.Stat(ctx, prev.Key)
	if err != nil {
		return err
	}
	snapshotImage, ok := sInfo.Labels[imageLabel]
	if !ok {
		return fmt.Errorf("snapshot %s does not have a service image label", prev.Key)
	}
	if snapshotImage == "" {
		return fmt.Errorf("snapshot %s has an empty service image label", prev.Key)
	}
	c.Image = snapshotImage
	c.SnapshotKey = prev.Key
	return nil
}

func newRevision(id string) *revision {
	now := time.Now()
	return &revision{
		Timestamp: now,
		Key:       fmt.Sprintf("boss.io.%s.%s", id, now.Format(timestampFormat)),
	}
}

type revision struct {
	Timestamp time.Time
	Key       string
	mounts    []mount.Mount
}

// nolint: interfacer
func create(ctx context.Context, client *containerd.Client, i containerd.Image, id string, previous string) (*revision, error) {
	diffIDs, err := i.RootFS(ctx)
	if err != nil {
		return nil, err
	}
	var (
		parent = identity.ChainID(diffIDs).String()
		r      = newRevision(id)
	)
	labels := map[string]string{
		gcRoot:     r.Timestamp.Format(time.RFC3339),
		imageLabel: i.Name(),
	}
	if previous != "" {
		labels[previousRevision] = previous
	}
	mounts, err := client.SnapshotService(containerd.DefaultSnapshotter).Prepare(ctx, r.Key, parent, snapshots.WithLabels(labels))
	if err != nil {
		return nil, err
	}
	r.mounts = mounts
	return r, nil
}

func save(ctx context.Context, client *containerd.Client, updatedImage containerd.Image, c *containers.Container) (*revision, error) {
	snapshot, err := create(ctx, client, updatedImage, c.ID, c.SnapshotKey)
	if err != nil {
		return nil, err
	}
	service := client.SnapshotService(c.Snapshotter)
	// create a diff from the existing snapshot
	diff, err := rootfs.CreateDiff(ctx, c.SnapshotKey, service, client.DiffService())
	if err != nil {
		return nil, err
	}
	applier := apply.NewFileSystemApplier(client.ContentStore())
	if _, err := applier.Apply(ctx, diff, snapshot.mounts); err != nil {
		return nil, err
	}
	return snapshot, nil
}

// nolint: interfacer
func previous(ctx context.Context, client *containerd.Client, c *containers.Container) (*revision, error) {
	service := client.SnapshotService(c.Snapshotter)
	sInfo, err := service.Stat(ctx, c.SnapshotKey)
	if err != nil {
		return nil, err
	}
	key := sInfo.Labels[previousRevision]
	if key == "" {
		return nil, ErrNoPreviousRevision
	}
	parts := strings.Split(key, ".")
	timestamp, err := time.Parse(timestampFormat, parts[len(parts)-1])
	if err != nil {
		return nil, err
	}
	return &revision{
		Timestamp: timestamp,
		Key:       key,
	}, nil
}

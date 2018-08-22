package image

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mitchellh/hashstructure"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/model"

	"github.com/docker/distribution/reference"
	digest "github.com/opencontainers/go-digest"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/windmilleng/wmclient/pkg/dirs"
)

// A monotonically increasing ID.
//
// For now, this is just a Time. In the future, this might be some better
// ID like an FSEvents event ID.
type CheckpointID time.Time

func (c CheckpointID) After(d CheckpointID) bool {
	return time.Time(c).After(time.Time(d))
}

func (c CheckpointID) MarshalJSON() ([]byte, error) {
	return time.Time(c).MarshalJSON()
}

func (c *CheckpointID) UnmarshalJSON(b []byte) error {
	if c == nil {
		return nil
	}
	t := time.Time(*c)
	err := (&t).UnmarshalJSON(b)
	*c = CheckpointID(t)
	return err
}

// Track all the images that tilt has ever built.
type ImageHistory struct {
	dir    *dirs.WindmillDir
	byName map[refKey]*NamedImageHistory
	mu     *sync.Mutex
}

func NewImageHistory(ctx context.Context, dir *dirs.WindmillDir) (ImageHistory, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-NewImageHistory")
	defer span.Finish()
	history := ImageHistory{
		dir:    dir,
		byName: make(map[refKey]*NamedImageHistory, 0),
		mu:     &sync.Mutex{},
	}
	entryMap, err := historyFromFS(ctx, dir)
	if err != nil {
		return ImageHistory{}, err
	}

	for ref, entries := range entryMap {
		name, err := reference.ParseNormalizedNamed(string(ref))
		if err != nil {
			return ImageHistory{}, fmt.Errorf("NewImageHistory: %v", err)
		}

		for _, entry := range entries {
			history.loadFromEntry(ctx, name, entry)
		}
	}

	return history, nil
}

// Create a new checkpoint ID.
//
// Clients should call this before they build an image, to ensure that the
// checkpoint captures all changes to the image before the current checkpoint.
func (h ImageHistory) CheckpointNow() CheckpointID {
	return CheckpointID(time.Now())
}

func (h ImageHistory) loadFromEntry(ctx context.Context, name reference.Named, entry historyEntry) {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := makeRefKey(name)
	bucket, ok := h.byName[key]
	if !ok {
		bucket = &NamedImageHistory{name: name}
		h.byName[key] = bucket
	}

	bucket.entries = append(bucket.entries, entry)
	if entry.After(bucket.mostRecent.CheckpointID) {
		bucket.mostRecent = entry
	}
}

func (h ImageHistory) load(
	ctx context.Context,
	name reference.Named,
	digest digest.Digest,
	checkpoint CheckpointID,
	baseDockerfile build.Dockerfile,
	mounts []model.Mount,
	steps []model.Cmd,
	entrypoint model.Cmd,
) (refKey, historyEntry, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := makeRefKey(name)
	bucket, ok := h.byName[key]
	if !ok {
		bucket = &NamedImageHistory{name: name}
		h.byName[key] = bucket
	}

	hi, err := hashInputs(baseDockerfile, mounts, steps, entrypoint)
	if err != nil {
		return "", historyEntry{}, err
	}

	entry := historyEntry{
		Digest:       digest,
		CheckpointID: checkpoint,
		HashedInputs: hi,
	}
	bucket.entries = append(bucket.entries, entry)
	if entry.After(bucket.mostRecent.CheckpointID) {
		bucket.mostRecent = entry
	}

	return key, entry, nil
}

type hash struct {
	BaseDockerfile build.Dockerfile
	Mounts         []model.Mount
	Steps          []model.Cmd
	Entrypoint     model.Cmd
}

type HashedInputs = uint64

func hashInputs(baseDockerfile build.Dockerfile, mounts []model.Mount, steps []model.Cmd, entrypoint model.Cmd) (HashedInputs, error) {
	hi := hash{
		BaseDockerfile: baseDockerfile,
		Mounts:         mounts,
		Steps:          steps,
		Entrypoint:     entrypoint,
	}

	hash, err := hashstructure.Hash(hi, nil)
	if err != nil {
		return 0, err
	}

	return HashedInputs(hash), nil
}

func (h ImageHistory) AddAndPersist(
	ctx context.Context,
	name reference.Named,
	digest digest.Digest,
	checkpoint CheckpointID,
	baseDockerfile build.Dockerfile,
	mounts []model.Mount,
	steps []model.Cmd,
	entrypoint model.Cmd,
) error {
	key, entry, err := h.load(ctx, name, digest, checkpoint, baseDockerfile, mounts, steps, entrypoint)
	if err != nil {
		return err
	}

	return addHistoryToFS(ctx, h.dir, key, entry)
}

func (h ImageHistory) MostRecent(
	name reference.Named,
	baseDockerfile build.Dockerfile,
	mounts []model.Mount,
	steps []model.Cmd,
	entrypoint model.Cmd,
) (digest.Digest, CheckpointID, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := makeRefKey(name)
	bucket, ok := h.byName[key]
	if !ok {
		return "", CheckpointID{}, false
	}

	hi, err := hashInputs(baseDockerfile, mounts, steps, entrypoint)
	if err != nil {
		// TODO(dmiller) return error here?
		return "", CheckpointID{}, false
	}

	mostRecent := bucket.mostRecent
	if mostRecent.HashedInputs != hi {
		return "", CheckpointID{}, false
	}
	return mostRecent.Digest, mostRecent.CheckpointID, true
}

type refKey string

func makeRefKey(name reference.Named) refKey {
	return refKey(name.String())
}

// Track all the images that tilt has ever built, indexed under a particular base name.
type NamedImageHistory struct {
	name       reference.Named
	entries    []historyEntry
	mostRecent historyEntry
}

type historyEntry struct {
	digest.Digest
	CheckpointID
	HashedInputs
}

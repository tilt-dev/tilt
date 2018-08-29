package image

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/windmilleng/tilt/internal/model"

	"github.com/docker/distribution/reference"
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

// NewImageHistory reads the persisted image history from disk and loads it in to memory
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
			history.addInMemoryFromEntry(ctx, name, entry)
		}
	}

	return history, nil
}

// CheckpointNow creates a new checkpoint ID.
//
// Clients should call this before they build an image, to ensure that the
// checkpoint captures all changes to the image before the current checkpoint.
func (h ImageHistory) CheckpointNow() CheckpointID {
	return CheckpointID(time.Now())
}

func (h ImageHistory) bucketAndKey(name reference.Named) (*NamedImageHistory, refKey) {
	key := makeRefKey(name)
	bucket, ok := h.byName[key]
	if !ok {
		bucket = &NamedImageHistory{name: name}
		h.byName[key] = bucket
	}

	return bucket, key
}

// addInMemoryFromEntry takes a historyEntry and adds it to the appropriate bucket in memory.
func (h ImageHistory) addInMemoryFromEntry(ctx context.Context, name reference.Named, entry historyEntry) {
	h.mu.Lock()
	defer h.mu.Unlock()

	bucket, _ := h.bucketAndKey(name)

	bucket.entries = append(bucket.entries, entry)
	if entry.After(bucket.mostRecent.CheckpointID) {
		bucket.mostRecent = entry
	}
}

// addInMemory takes checkpoint and a service definition and loads it in to the appropriate bucket in memory.
func (h ImageHistory) addInMemory(
	ctx context.Context,
	name reference.Named,
	checkpoint CheckpointID,
	service model.Service,
) (refKey, historyEntry, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	bucket, key := h.bucketAndKey(name)

	hash, err := service.Hash()
	if err != nil {
		return "", historyEntry{}, err
	}

	entry := historyEntry{
		Named:         name,
		CheckpointID:  checkpoint,
		HashedService: hash,
	}
	bucket.entries = append(bucket.entries, entry)
	if entry.After(bucket.mostRecent.CheckpointID) {
		bucket.mostRecent = entry
	}

	return key, entry, nil
}

// AddAndPersist takes a checkpoint and a service, loads it in to memory and persists it to disk
func (h ImageHistory) AddAndPersist(
	ctx context.Context,
	name reference.Named,
	checkpoint CheckpointID,
	service model.Service,
) error {
	key, entry, err := h.addInMemory(ctx, name, checkpoint, service)
	if err != nil {
		return err
	}

	return addHistoryToFS(ctx, h.dir, key, entry)
}

// MostRecent returns the most recent image for a given image and service, or nothing if the service changed
func (h ImageHistory) MostRecent(
	name reference.Named,
	service model.Service,
) (reference.Named, CheckpointID, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := makeRefKey(name)
	bucket, ok := h.byName[key]
	if !ok {
		return nil, CheckpointID{}, false
	}

	hash, err := service.Hash()
	if err != nil {
		// TODO(dmiller) return error here?
		return nil, CheckpointID{}, false
	}

	mostRecent := bucket.mostRecent
	if mostRecent.HashedService != hash {
		return nil, CheckpointID{}, false
	}

	return mostRecent.Named, mostRecent.CheckpointID, true
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
	reference.Named
	CheckpointID
	model.HashedService
}

package image

import (
	"sync"
	"time"

	"github.com/docker/distribution/reference"
	digest "github.com/opencontainers/go-digest"
)

// A monotonically increasing ID.
//
// For now, this is just a Time. In the future, this might be some better
// ID like an FSEvents event ID.
type CheckpointID time.Time

func (c CheckpointID) After(d CheckpointID) bool {
	return time.Time(c).After(time.Time(d))
}

// Track all the images that tilt has ever built.
type ImageHistory struct {
	byName map[refKey]*NamedImageHistory
	mu     *sync.Mutex
}

func NewImageHistory() ImageHistory {
	return ImageHistory{
		byName: make(map[refKey]*NamedImageHistory, 0),
		mu:     &sync.Mutex{},
	}
}

// Create a new checkpoint ID.
//
// Clients should call this before they build an image, to ensure that the
// checkpoint captures all changes to the image before the current checkpoint.
func (h ImageHistory) CheckpointNow() CheckpointID {
	return CheckpointID(time.Now())
}

func (h ImageHistory) Add(name reference.Named, digest digest.Digest, checkpoint CheckpointID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := makeRefKey(name)
	bucket, ok := h.byName[key]
	if !ok {
		bucket = &NamedImageHistory{name: name}
		h.byName[key] = bucket
	}

	entry := historyEntry{
		Digest:       digest,
		CheckpointID: checkpoint,
	}
	bucket.entries = append(bucket.entries, entry)
	if entry.After(bucket.mostRecent.CheckpointID) {
		bucket.mostRecent = entry
	}
}

func (h ImageHistory) MostRecent(name reference.Named) (digest.Digest, CheckpointID, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := makeRefKey(name)
	bucket, ok := h.byName[key]
	if !ok {
		return "", CheckpointID{}, false
	}

	mostRecent := bucket.mostRecent
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
}

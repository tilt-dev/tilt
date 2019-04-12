package store

import (
	"sync"
	"time"
)

// A little synchronization primitive that comes up frequently in build systems,
// with three assumptions:
//
// 1) An event can come in at any time that marks the status as "dirty"
//    (think: the user edits a file).
// 2) Builds can take a long time.
// 3) When the build finishes, we want to mark the status as "clean" iff
//    nothing has changed since the build started.
//
// Don't use this primitive if you do synchronization at a higher
// level (as EngineState does), or need more granular dirtyness tracking
// (as EngineState also does, see PendingFileChanges). But EngineState
// uses a similar architecture internally.

type DirtyBit struct {
	mu        sync.Mutex
	dirtyAsOf time.Time
}

func NewDirtyBit() *DirtyBit {
	return &DirtyBit{}
}

// Mark the bit as dirty.
// If the change happens and this is marked dirty later, that's usually ok.
// It just means IsDirty might have false positives (i.e., we do spurious builds).
func (b *DirtyBit) MarkDirty() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.dirtyAsOf = time.Now()
}

func (b *DirtyBit) IsDirty() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	return !b.dirtyAsOf.IsZero()
}

// If the bit is currently marked dirty, returns a StartToken
// to pass to FinishBuild. Otherwise, return false.
func (b *DirtyBit) StartBuildIfDirty() (DirtyStartToken, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.dirtyAsOf.IsZero() {
		return DirtyStartToken{}, false
	}

	return DirtyStartToken(time.Now()), true
}

func (b *DirtyBit) FinishBuild(t DirtyStartToken) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if time.Time(b.dirtyAsOf).After(time.Time(t)) {
		return
	}
	b.dirtyAsOf = time.Time{}
}

type DirtyStartToken time.Time

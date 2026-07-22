package store

import (
	"time"

	"k8s.io/apimachinery/pkg/types"
)

// Represents all the IDs of a particular type of resource
// that have changed.
type ChangeSet struct {
	Changes map[types.NamespacedName]bool
}

func NewChangeSet(names ...types.NamespacedName) ChangeSet {
	cs := ChangeSet{}
	for _, name := range names {
		cs.Add(name)
	}
	return cs
}

func (s *ChangeSet) Empty() bool {
	return len(s.Changes) == 0
}

// Add a changed resource name.
func (s *ChangeSet) Add(nn types.NamespacedName) {
	if s.Changes == nil {
		s.Changes = make(map[types.NamespacedName]bool)
	}
	s.Changes[nn] = true
}

// Merge another change set into this one.
func (s *ChangeSet) AddAll(other ChangeSet) {
	if len(other.Changes) > 0 {
		if s.Changes == nil {
			s.Changes = make(map[types.NamespacedName]bool)
		}
		for k, v := range other.Changes {
			s.Changes[k] = v
		}
	}
}

// Summarize the changes to the EngineState since the last change.
type ChangeSummary struct {
	// True if we saw one or more legacy actions that don't know how
	// to summarize their changes.
	Legacy bool

	// True if this change added logs.
	Log bool

	// Cmds with their specs changed.
	CmdSpecs ChangeSet

	UISessions  ChangeSet
	UIResources ChangeSet
	UIButtons   ChangeSet

	Clusters ChangeSet

	// If non-zero, that means we tried to apply this change and got
	// an error.
	LastBackoff time.Duration
}

func (s ChangeSummary) IsLogOnly() bool {
	// Keep this equivalent to the exact value ChangeSummary{Log: true}.
	// In particular, a log flag does not make a mixed summary log-only.
	return s.Log &&
		!s.Legacy &&
		s.CmdSpecs.Changes == nil &&
		s.UISessions.Changes == nil &&
		s.UIResources.Changes == nil &&
		s.UIButtons.Changes == nil &&
		s.Clusters.Changes == nil &&
		s.LastBackoff == 0
}

func (s *ChangeSummary) Add(other ChangeSummary) {
	s.Legacy = s.Legacy || other.Legacy
	s.Log = s.Log || other.Log
	s.CmdSpecs.AddAll(other.CmdSpecs)
	s.UISessions.AddAll(other.UISessions)
	s.UIResources.AddAll(other.UIResources)
	s.UIButtons.AddAll(other.UIButtons)
	s.Clusters.AddAll(other.Clusters)
	if other.LastBackoff > s.LastBackoff {
		s.LastBackoff = other.LastBackoff
	}
}

func LegacyChangeSummary() ChangeSummary {
	return ChangeSummary{Legacy: true}
}

type Summarizer interface {
	Action

	Summarize(summary *ChangeSummary)
}

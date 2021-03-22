package store

import (
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/types"
)

// Summarize the changes to the EngineState since the last change.
type ChangeSummary struct {
	// True if we saw one or more legacy actions that don't know how
	// to summarize their changes.
	Legacy bool

	// True if this change added logs.
	Log bool

	// Cmds with their specs changed.
	CmdSpecs map[string]bool

	// FileWatches with their specs changed.
	FileWatchSpecs map[types.NamespacedName]bool
}

func (s ChangeSummary) IsLogOnly() bool {
	return cmp.Equal(s, ChangeSummary{Log: true})
}

func (s *ChangeSummary) Add(other ChangeSummary) {
	s.Legacy = s.Legacy || other.Legacy
	s.Log = s.Log || other.Log
	if len(other.CmdSpecs) > 0 {
		if s.CmdSpecs == nil {
			s.CmdSpecs = make(map[string]bool)
		}
		for k, v := range other.CmdSpecs {
			s.CmdSpecs[k] = v
		}
	}
	if len(other.FileWatchSpecs) > 0 {
		if s.FileWatchSpecs == nil {
			s.FileWatchSpecs = make(map[types.NamespacedName]bool)
		}
		for k, v := range other.FileWatchSpecs {
			s.FileWatchSpecs[k] = v
		}
	}
}

func LegacyChangeSummary() ChangeSummary {
	return ChangeSummary{Legacy: true}
}

type Summarizer interface {
	Action

	Summarize(summary *ChangeSummary)
}

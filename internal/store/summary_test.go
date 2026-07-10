package store

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
)

func TestChangeSummaryIsLogOnly(t *testing.T) {
	changed := NewChangeSet(types.NamespacedName{Name: "changed"})
	emptyButAllocated := ChangeSet{Changes: make(map[types.NamespacedName]bool)}

	tests := []struct {
		name    string
		summary ChangeSummary
		want    bool
	}{
		{name: "exact log only", summary: ChangeSummary{Log: true}, want: true},
		{name: "zero summary", summary: ChangeSummary{}, want: false},
		{name: "legacy", summary: ChangeSummary{Log: true, Legacy: true}, want: false},
		{name: "command specs", summary: ChangeSummary{Log: true, CmdSpecs: changed}, want: false},
		{name: "UI sessions", summary: ChangeSummary{Log: true, UISessions: changed}, want: false},
		{name: "UI resources", summary: ChangeSummary{Log: true, UIResources: changed}, want: false},
		{name: "UI buttons", summary: ChangeSummary{Log: true, UIButtons: changed}, want: false},
		{name: "clusters", summary: ChangeSummary{Log: true, Clusters: changed}, want: false},
		{name: "last backoff", summary: ChangeSummary{Log: true, LastBackoff: time.Second}, want: false},

		// cmp.Equal distinguishes a nil map from an allocated empty map. Keep that
		// exact-value behavior while replacing reflection in the hot path.
		{name: "allocated empty command specs", summary: ChangeSummary{Log: true, CmdSpecs: emptyButAllocated}, want: false},
		{name: "allocated empty UI sessions", summary: ChangeSummary{Log: true, UISessions: emptyButAllocated}, want: false},
		{name: "allocated empty UI resources", summary: ChangeSummary{Log: true, UIResources: emptyButAllocated}, want: false},
		{name: "allocated empty UI buttons", summary: ChangeSummary{Log: true, UIButtons: emptyButAllocated}, want: false},
		{name: "allocated empty clusters", summary: ChangeSummary{Log: true, Clusters: emptyButAllocated}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.summary.IsLogOnly())
		})
	}
}

func TestChangeSummaryAdd(t *testing.T) {
	name := func(s string) types.NamespacedName { return types.NamespacedName{Name: s} }

	t.Run("merges every field", func(t *testing.T) {
		dst := ChangeSummary{}
		src := ChangeSummary{
			Legacy:      true,
			Log:         true,
			CmdSpecs:    NewChangeSet(name("cmd")),
			UISessions:  NewChangeSet(name("session")),
			UIResources: NewChangeSet(name("resource")),
			UIButtons:   NewChangeSet(name("button")),
			Clusters:    NewChangeSet(name("cluster")),
			LastBackoff: time.Second,
		}
		dst.Add(src)
		assert.Equal(t, src, dst)
	})

	t.Run("keeps larger backoff", func(t *testing.T) {
		dst := ChangeSummary{LastBackoff: 2 * time.Second}
		dst.Add(ChangeSummary{LastBackoff: time.Second})
		assert.Equal(t, 2*time.Second, dst.LastBackoff)
	})
}

func TestChangeSummaryFieldCoverageGuard(t *testing.T) {
	// IsLogOnly and Add enumerate every ChangeSummary field by hand instead of
	// using reflection. If this count changes, update both methods (and their
	// tests above) before updating the count, or the new field is silently
	// ignored by log-only detection and summary merging.
	assert.Equal(t, 8, reflect.TypeOf(ChangeSummary{}).NumField())
}

func BenchmarkChangeSummaryIsLogOnly(b *testing.B) {
	summary := ChangeSummary{Log: true}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = summary.IsLogOnly()
	}
}

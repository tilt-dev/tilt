package client

import (
	"fmt"
	"testing"
	"time"

	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

// The filter every default `tilt up` terminal stream runs with: no
// constraint on any axis.
func noopBenchFilter() LogFilter {
	return NewLogFilter(FilterSourceAll, nil, FilterLevel(logger.NoneLvl),
		FilterSince(time.Time{}), FilterTail(-1), false)
}

func benchFilterLines(n int) []logstore.LogLine {
	ts := time.Unix(1594000000, 0)
	lines := make([]logstore.LogLine, 0, n)
	for i := 0; i < n; i++ {
		mn := model.ManifestName(fmt.Sprintf("res-%03d", i%20))
		lines = append(lines, logstore.LogLine{
			Text:         fmt.Sprintf("log line %d with a typical payload\n", i),
			SpanID:       logstore.SpanID(fmt.Sprintf("pod:default:%s", mn)),
			ManifestName: mn,
			Level:        logger.InfoLvl,
			Time:         ts,
		})
	}
	return lines
}

// The steady-state case: a no-op filter applied to each incremental batch
// on every store notification.
func BenchmarkLogFilterApplyNoop(b *testing.B) {
	f := noopBenchFilter()
	lines := benchFilterLines(50)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = f.Apply(lines)
	}
}

// The initial-history case: a no-op filter over a large first batch.
func BenchmarkLogFilterApplyNoopLarge(b *testing.B) {
	f := noopBenchFilter()
	lines := benchFilterLines(2000)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = f.Apply(lines)
	}
}

// A real filter that keeps a subset (one resource out of twenty).
func BenchmarkLogFilterApplyResourceFilter(b *testing.B) {
	f := NewLogFilter(FilterSourceAll, FilterResources{"res-001"},
		FilterLevel(logger.NoneLvl), FilterSince(time.Time{}), FilterTail(-1), false)
	lines := benchFilterLines(2000)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = f.Apply(lines)
	}
}

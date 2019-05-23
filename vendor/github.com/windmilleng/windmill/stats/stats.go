// Metrics and middleware
//
// Currently sends to datadog's statsd daemon on each node.

package stats

import (
	"io"
	"time"
)

type StatsReporter interface {
	io.Closer
	Timing(name string, value time.Duration, tags map[string]string, rate float64) error
	Count(name string, value int64, tags map[string]string, rate float64) error
	Incr(name string, tags map[string]string, rate float64) error
}

func NewBlackholeStatsReporter() blackholeStatsReporter {
	return blackholeStatsReporter{}
}

type blackholeStatsReporter struct {
}

func (r blackholeStatsReporter) Close() error {
	return nil
}

func (r blackholeStatsReporter) Timing(name string, value time.Duration, tags map[string]string, rate float64) error {
	return nil
}

func (r blackholeStatsReporter) Count(name string, value int64, tags map[string]string, rate float64) error {
	return nil
}

func (r blackholeStatsReporter) Incr(name string, tags map[string]string, rate float64) error {
	return nil
}

var _ StatsReporter = blackholeStatsReporter{}

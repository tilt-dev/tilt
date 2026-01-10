package hud

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

func TestLogFilterMatches(t *testing.T) {
	type testCase struct {
		description string
		logFilter   LogFilter
		input       logstore.LogLine
		expected    bool
	}
	testCases := []testCase{
		{
			description: "source all matches pod logs",
			logFilter:   LogFilter{source: FilterSourceAll},
			input:       logstore.LogLine{SpanID: "pod:default:nginx"},
			expected:    true,
		},
		{
			description: "source all matches build logs",
			logFilter:   LogFilter{source: FilterSourceAll},
			input:       logstore.LogLine{SpanID: "build:1"},
			expected:    true,
		},
		{
			description: "source all matches cmdimage logs",
			logFilter:   LogFilter{source: FilterSourceBuild},
			input:       logstore.LogLine{SpanID: "cmdimage:nginx"},
			expected:    true,
		},
		{
			description: "source all matches tiltfile logs",
			logFilter:   LogFilter{source: FilterSourceAll},
			input:       logstore.LogLine{SpanID: "tiltfile:(Tiltfile):1"},
			expected:    true,
		},
		{
			description: "source build does not match pod logs",
			logFilter:   LogFilter{source: FilterSourceBuild},
			input:       logstore.LogLine{SpanID: "pod:default:nginx"},
			expected:    false,
		},
		{
			description: "source build matches build logs",
			logFilter:   LogFilter{source: FilterSourceBuild},
			input:       logstore.LogLine{SpanID: "build:1"},
			expected:    true,
		},
		{
			description: "source build matches cmdimage logs",
			logFilter:   LogFilter{source: FilterSourceBuild},
			input:       logstore.LogLine{SpanID: "cmdimage:nginx"},
			expected:    true,
		},
		{
			description: "source build does not match tiltfile logs",
			logFilter:   LogFilter{source: FilterSourceBuild},
			input:       logstore.LogLine{SpanID: "tiltfile:(Tiltfile):1"},
			expected:    false,
		},
		{
			description: "source runtime matches pod logs",
			logFilter:   LogFilter{source: FilterSourceRuntime},
			input:       logstore.LogLine{SpanID: "pod:default:nginx"},
			expected:    true,
		},
		{
			description: "source runtime does not match build logs",
			logFilter:   LogFilter{source: FilterSourceRuntime},
			input:       logstore.LogLine{SpanID: "build:1"},
			expected:    false,
		},
		{
			description: "source runtime does not match cmdimage logs",
			logFilter:   LogFilter{source: FilterSourceRuntime},
			input:       logstore.LogLine{SpanID: "cmdimage:nginx"},
			expected:    false,
		},
		{
			description: "source runtime matches tiltfile logs",
			logFilter:   LogFilter{source: FilterSourceRuntime},
			input:       logstore.LogLine{SpanID: "tiltfile:(Tiltfile):1"},
			expected:    true,
		},
		{
			description: "source all matches logs with buildEvent",
			logFilter:   LogFilter{source: FilterSourceAll},
			input:       logstore.LogLine{SpanID: "pod:default:nginx", BuildEvent: "init"},
			expected:    true,
		},
		{
			description: "source build matches logs with buildEvent",
			logFilter:   LogFilter{source: FilterSourceBuild},
			input:       logstore.LogLine{SpanID: "pod:default:nginx", BuildEvent: "init"},
			expected:    true,
		},
		{
			description: "source runtime matches logs with buildEvent",
			logFilter:   LogFilter{source: FilterSourceRuntime},
			input:       logstore.LogLine{SpanID: "build:1", BuildEvent: "init"},
			expected:    true,
		},
		{
			description: "level lower than warn matches logs with any level",
			logFilter:   LogFilter{source: FilterSourceAll, level: logger.NoneLvl},
			input:       logstore.LogLine{SpanID: "pod:default:nginx", Level: logger.InfoLvl},
			expected:    true,
		},
		{
			description: "level warn matches logs with warn level",
			logFilter:   LogFilter{source: FilterSourceAll, level: logger.WarnLvl},
			input:       logstore.LogLine{SpanID: "pod:default:nginx", Level: logger.WarnLvl},
			expected:    true,
		},
		{
			description: "level warn does not match logs with error level",
			logFilter:   LogFilter{source: FilterSourceAll, level: logger.WarnLvl},
			input:       logstore.LogLine{SpanID: "pod:default:nginx", Level: logger.ErrorLvl},
			expected:    false,
		},
		{
			description: "level error matches logs with error level",
			logFilter:   LogFilter{source: FilterSourceAll, level: logger.ErrorLvl},
			input:       logstore.LogLine{SpanID: "pod:default:nginx", Level: logger.ErrorLvl},
			expected:    true,
		},
		{
			description: "level error does not match logs with warn level",
			logFilter:   LogFilter{source: FilterSourceAll, level: logger.ErrorLvl},
			input:       logstore.LogLine{SpanID: "pod:default:nginx", Level: logger.WarnLvl},
			expected:    false,
		},
		{
			description: "empty manifest name matches any logs",
			logFilter:   LogFilter{source: FilterSourceAll, resources: nil},
			input:       logstore.LogLine{SpanID: "pod:default:nginx", ManifestName: "nginx"},
			expected:    true,
		},
		{
			description: "manifest name matches logs with the same manifest name",
			logFilter:   LogFilter{source: FilterSourceAll, resources: FilterResources{"nginx"}},
			input:       logstore.LogLine{SpanID: "pod:default:nginx", ManifestName: "nginx"},
			expected:    true,
		},
		{
			description: "manifest name does not match logs with a different manifest name",
			logFilter:   LogFilter{source: FilterSourceAll, resources: FilterResources{"test"}},
			input:       logstore.LogLine{SpanID: "pod:default:nginx", ManifestName: "nginx"},
			expected:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			actual := tc.logFilter.Matches(tc.input)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestLogFilterApply(t *testing.T) {
	type testCase struct {
		description string
		logFilter   LogFilter
		input       []logstore.LogLine
		expected    []logstore.LogLine
	}
	testCases := []testCase{
		{
			description: "empty filter matches all logs",
			logFilter:   LogFilter{tail: -1},
			input: []logstore.LogLine{
				{SpanID: "pod:default:nginx"},
				{SpanID: "build:1"},
				{SpanID: "cmdimage:nginx"},
				{SpanID: "tiltfile:(Tiltfile):1"},
			},
			expected: []logstore.LogLine{
				{SpanID: "pod:default:nginx"},
				{SpanID: "build:1"},
				{SpanID: "cmdimage:nginx"},
				{SpanID: "tiltfile:(Tiltfile):1"},
			},
		},
		{
			description: "filter with source all matches all logs",
			logFilter:   LogFilter{source: FilterSourceAll, tail: -1},
			input: []logstore.LogLine{
				{SpanID: "pod:default:nginx"},
				{SpanID: "build:1"},
				{SpanID: "cmdimage:nginx"},
				{SpanID: "tiltfile:(Tiltfile):1"},
			},
			expected: []logstore.LogLine{
				{SpanID: "pod:default:nginx"},
				{SpanID: "build:1"},
				{SpanID: "cmdimage:nginx"},
				{SpanID: "tiltfile:(Tiltfile):1"},
			},
		},
		{
			description: "filter with source build matches build logs",
			logFilter:   LogFilter{source: FilterSourceBuild, tail: -1},
			input: []logstore.LogLine{
				{SpanID: "pod:default:nginx"},
				{SpanID: "build:1"},
				{SpanID: "cmdimage:nginx"},
				{SpanID: "tiltfile:(Tiltfile):1"},
			},
			expected: []logstore.LogLine{
				{SpanID: "build:1"},
				{SpanID: "cmdimage:nginx"},
			},
		},
		{
			description: "filter with source runtime matches non-build logs",
			logFilter:   LogFilter{source: FilterSourceRuntime, tail: -1},
			input: []logstore.LogLine{
				{SpanID: "pod:default:nginx"},
				{SpanID: "build:1"},
				{SpanID: "cmdimage:nginx"},
				{SpanID: "tiltfile:(Tiltfile):1"},
			},
			expected: []logstore.LogLine{
				{SpanID: "pod:default:nginx"},
				{SpanID: "tiltfile:(Tiltfile):1"},
			},
		},
		{
			description: "filter with level warn matches warn logs",
			logFilter:   LogFilter{source: FilterSourceAll, level: logger.WarnLvl, tail: -1},
			input: []logstore.LogLine{
				{SpanID: "pod:default:nginx", Level: logger.DebugLvl},
				{SpanID: "build:1", Level: logger.InfoLvl},
				{SpanID: "cmdimage:nginx", Level: logger.WarnLvl},
				{SpanID: "tiltfile:(Tiltfile):1", Level: logger.ErrorLvl},
			},
			expected: []logstore.LogLine{
				{SpanID: "cmdimage:nginx", Level: logger.WarnLvl},
			},
		},
		{
			description: "filter with level error matches error logs",
			logFilter:   LogFilter{source: FilterSourceAll, level: logger.ErrorLvl, tail: -1},
			input: []logstore.LogLine{
				{SpanID: "pod:default:nginx", Level: logger.DebugLvl},
				{SpanID: "build:1", Level: logger.InfoLvl},
				{SpanID: "cmdimage:nginx", Level: logger.WarnLvl},
				{SpanID: "tiltfile:(Tiltfile):1", Level: logger.ErrorLvl},
			},
			expected: []logstore.LogLine{
				{SpanID: "tiltfile:(Tiltfile):1", Level: logger.ErrorLvl},
			},
		},
		{
			description: "filter with manifest name matches only logs with the same manifest name",
			logFilter:   LogFilter{source: FilterSourceAll, resources: FilterResources{"nginx"}, tail: -1},
			input: []logstore.LogLine{
				{SpanID: "pod:default:nginx", ManifestName: "nginx"},
				{SpanID: "build:1", ManifestName: "nginx"},
				{SpanID: "cmdimage:nginx", ManifestName: "nginx"},
				{SpanID: "tiltfile:(Tiltfile):1", ManifestName: "(Tiltfile)"},
			},
			expected: []logstore.LogLine{
				{SpanID: "pod:default:nginx", ManifestName: "nginx"},
				{SpanID: "build:1", ManifestName: "nginx"},
				{SpanID: "cmdimage:nginx", ManifestName: "nginx"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			actual := tc.logFilter.Apply(tc.input)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestLogFilterApplyWithSince(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		description string
		since       time.Time // cutoff timestamp (zero means no filter)
		input       []logstore.LogLine
		expected    []logstore.LogLine
	}{
		{
			description: "zero since returns all logs",
			since:       time.Time{},
			input: []logstore.LogLine{
				{SpanID: "pod:1", Time: now.Add(-1 * time.Hour)},
				{SpanID: "pod:2", Time: now.Add(-30 * time.Minute)},
				{SpanID: "pod:3", Time: now.Add(-5 * time.Minute)},
			},
			expected: []logstore.LogLine{
				{SpanID: "pod:1", Time: now.Add(-1 * time.Hour)},
				{SpanID: "pod:2", Time: now.Add(-30 * time.Minute)},
				{SpanID: "pod:3", Time: now.Add(-5 * time.Minute)},
			},
		},
		{
			description: "since 10m ago filters logs older than 10 minutes",
			since:       now.Add(-10 * time.Minute),
			input: []logstore.LogLine{
				{SpanID: "pod:1", Time: now.Add(-1 * time.Hour)},
				{SpanID: "pod:2", Time: now.Add(-30 * time.Minute)},
				{SpanID: "pod:3", Time: now.Add(-5 * time.Minute)},
			},
			expected: []logstore.LogLine{
				{SpanID: "pod:3", Time: now.Add(-5 * time.Minute)},
			},
		},
		{
			description: "since 1h ago includes logs at boundary",
			since:       now.Add(-1 * time.Hour),
			input: []logstore.LogLine{
				{SpanID: "pod:1", Time: now.Add(-1 * time.Hour)},
				{SpanID: "pod:2", Time: now.Add(-30 * time.Minute)},
			},
			expected: []logstore.LogLine{
				{SpanID: "pod:1", Time: now.Add(-1 * time.Hour)},
				{SpanID: "pod:2", Time: now.Add(-30 * time.Minute)},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			filter := LogFilter{since: tc.since, tail: -1}
			actual := filter.Apply(tc.input)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestLogFilterApplyWithTail(t *testing.T) {
	testCases := []struct {
		description string
		tail        int
		input       []logstore.LogLine
		expected    []logstore.LogLine
	}{
		{
			description: "tail -1 returns all logs",
			tail:        -1,
			input: []logstore.LogLine{
				{SpanID: "pod:1"},
				{SpanID: "pod:2"},
				{SpanID: "pod:3"},
				{SpanID: "pod:4"},
			},
			expected: []logstore.LogLine{
				{SpanID: "pod:1"},
				{SpanID: "pod:2"},
				{SpanID: "pod:3"},
				{SpanID: "pod:4"},
			},
		},
		{
			description: "tail 2 returns last 2 logs",
			tail:        2,
			input: []logstore.LogLine{
				{SpanID: "pod:1"},
				{SpanID: "pod:2"},
				{SpanID: "pod:3"},
				{SpanID: "pod:4"},
			},
			expected: []logstore.LogLine{
				{SpanID: "pod:3"},
				{SpanID: "pod:4"},
			},
		},
		{
			description: "tail 0 returns no logs",
			tail:        0,
			input: []logstore.LogLine{
				{SpanID: "pod:1"},
				{SpanID: "pod:2"},
			},
			expected: []logstore.LogLine{},
		},
		{
			description: "tail larger than input returns all",
			tail:        10,
			input: []logstore.LogLine{
				{SpanID: "pod:1"},
				{SpanID: "pod:2"},
			},
			expected: []logstore.LogLine{
				{SpanID: "pod:1"},
				{SpanID: "pod:2"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			filter := LogFilter{tail: tc.tail}
			actual := filter.Apply(tc.input)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestLogFilterApplyWithSinceAndTail(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	// Test chaining: since filters first, then tail takes from the result
	input := []logstore.LogLine{
		{SpanID: "pod:1", Time: now.Add(-2 * time.Hour)}, // too old
		{SpanID: "pod:2", Time: now.Add(-45 * time.Minute)},
		{SpanID: "pod:3", Time: now.Add(-30 * time.Minute)},
		{SpanID: "pod:4", Time: now.Add(-15 * time.Minute)},
		{SpanID: "pod:5", Time: now.Add(-5 * time.Minute)},
	}

	filter := LogFilter{
		since: now.Add(-1 * time.Hour), // cutoff: 1 hour ago, filters to pod:2, pod:3, pod:4, pod:5
		tail:  2,                       // takes last 2: pod:4, pod:5
	}

	expected := []logstore.LogLine{
		{SpanID: "pod:4", Time: now.Add(-15 * time.Minute)},
		{SpanID: "pod:5", Time: now.Add(-5 * time.Minute)},
	}

	actual := filter.Apply(input)
	assert.Equal(t, expected, actual)
}

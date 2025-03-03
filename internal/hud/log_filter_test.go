package hud

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
			description: "allow",
			logFilter:   LogFilter{spanPrefix: "pod"},
			input:       logstore.LogLine{SpanID: "pod:default:nginx"},
			expected:    true,
		},
		{
			description: "allow empty",
			logFilter:   LogFilter{spanPrefix: ""},
			input:       logstore.LogLine{SpanID: "pod:default:nginx"},
			expected:    true,
		},
		{
			description: "allow specific",
			logFilter:   LogFilter{spanPrefix: "pod:default"},
			input:       logstore.LogLine{SpanID: "pod:default:nginx"},
			expected:    true,
		},
		{
			description: "allow does not match",
			logFilter:   LogFilter{spanPrefix: "pod"},
			input:       logstore.LogLine{SpanID: "monitor:default:nginx"},
			expected:    false,
		},
		{
			description: "deny",
			logFilter:   LogFilter{spanPrefix: "pod", not: true},
			input:       logstore.LogLine{SpanID: "pod:default:nginx"},
			expected:    false,
		},
		{
			description: "deny empty",
			logFilter:   LogFilter{spanPrefix: "", not: true},
			input:       logstore.LogLine{SpanID: "pod:default:nginx"},
			expected:    false,
		},
		{
			description: "deny specific",
			logFilter:   LogFilter{spanPrefix: "pod:default", not: true},
			input:       logstore.LogLine{SpanID: "pod:my-namespace:nginx"},
			expected:    true,
		},
		{
			description: "deny does not match",
			logFilter:   LogFilter{spanPrefix: "pod", not: true},
			input:       logstore.LogLine{SpanID: "monitor:default:nginx"},
			expected:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			actual := tc.logFilter.Matches(tc.input)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestLogFiltersApply(t *testing.T) {
	type testCase struct {
		description string
		logFilters  LogFilters
		input       []logstore.LogLine
		expected    []logstore.LogLine
	}
	testCases := []testCase{
		{
			description: "no filters",
			logFilters:  LogFilters{},
			input: []logstore.LogLine{
				{SpanID: "pod:default:nginx"},
				{SpanID: "monitor:default:nginx"},
				{SpanID: "tiltfile:(Tiltfile):1"},
				{SpanID: "build:1"},
			},
			expected: []logstore.LogLine{
				{SpanID: "pod:default:nginx"},
				{SpanID: "monitor:default:nginx"},
				{SpanID: "tiltfile:(Tiltfile):1"},
				{SpanID: "build:1"},
			},
		},
		{
			description: "empty filters",
			logFilters: LogFilters{
				deny: []LogFilter{
					{spanPrefix: "", not: true},
				},
				allow: []LogFilter{
					{spanPrefix: "", not: false},
				},
			},
			input: []logstore.LogLine{
				{SpanID: "pod:default:nginx"},
				{SpanID: "monitor:default:nginx"},
				{SpanID: "tiltfile:(Tiltfile):1"},
				{SpanID: "build:1"},
			},
			expected: []logstore.LogLine{
				{SpanID: "pod:default:nginx"},
				{SpanID: "monitor:default:nginx"},
				{SpanID: "tiltfile:(Tiltfile):1"},
				{SpanID: "build:1"},
			},
		},
		{
			description: "multiple allow filters",
			logFilters: LogFilters{
				allow: []LogFilter{
					{spanPrefix: "pod"},
					{spanPrefix: "monitor"},
				},
			},
			input: []logstore.LogLine{
				{SpanID: "pod:default:nginx"},
				{SpanID: "monitor:default:nginx"},
				{SpanID: "tiltfile:(Tiltfile):1"},
				{SpanID: "build:1"},
			},
			expected: []logstore.LogLine{
				{SpanID: "pod:default:nginx"},
				{SpanID: "monitor:default:nginx"},
			},
		},
		{
			description: "multiple deny filters",
			logFilters: LogFilters{
				deny: []LogFilter{
					{spanPrefix: "pod", not: true},
					{spanPrefix: "monitor", not: true},
				},
			},
			input: []logstore.LogLine{
				{SpanID: "pod:default:nginx"},
				{SpanID: "monitor:default:nginx"},
				{SpanID: "tiltfile:(Tiltfile):1"},
				{SpanID: "build:1"},
			},
			expected: []logstore.LogLine{
				{SpanID: "tiltfile:(Tiltfile):1"},
				{SpanID: "build:1"},
			},
		},
		{
			description: "deny all and allow a subset",
			logFilters: LogFilters{
				deny: []LogFilter{
					{spanPrefix: "pod", not: true},
				},
				allow: []LogFilter{
					{spanPrefix: "pod:my-namespace"},
				},
			},
			input: []logstore.LogLine{
				{SpanID: "pod:default:nginx"},
				{SpanID: "pod:default:my-pod"},
				{SpanID: "pod:my-namespace:nginx"},
				{SpanID: "pod:my-namespace:my-pod"},
			},
			expected: []logstore.LogLine{
				{SpanID: "pod:my-namespace:nginx"},
				{SpanID: "pod:my-namespace:my-pod"},
			},
		},
		{
			description: "deny all and allow one",
			logFilters: LogFilters{
				deny: []LogFilter{
					{spanPrefix: "pod", not: true},
				},
				allow: []LogFilter{
					{spanPrefix: "pod:my-namespace:my-pod"},
				},
			},
			input: []logstore.LogLine{
				{SpanID: "pod:default:nginx"},
				{SpanID: "pod:default:my-pod"},
				{SpanID: "pod:my-namespace:nginx"},
				{SpanID: "pod:my-namespace:my-pod"},
			},
			expected: []logstore.LogLine{
				{SpanID: "pod:my-namespace:my-pod"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			actual := tc.logFilters.Apply(tc.input)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestLogFilterFromString(t *testing.T) {
	type testCase struct {
		description string
		input       string
		expected    LogFilter
	}

	testCases := []testCase{
		{
			description: "empty allow filter",
			input:       "",
			expected:    LogFilter{spanPrefix: "", not: false},
		},
		{
			description: "allow filter",
			input:       "pod",
			expected:    LogFilter{spanPrefix: "pod", not: false},
		},
		{
			description: "empty deny filter",
			input:       "!",
			expected:    LogFilter{spanPrefix: "", not: true},
		},
		{
			description: "deny filter",
			input:       "!pod",
			expected:    LogFilter{spanPrefix: "pod", not: true},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			actual := LogFilterFromString(tc.input)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestLogFiltersFromStrings(t *testing.T) {
	type testCase struct {
		description string
		input       []string
		expected    LogFilters
	}

	testCases := []testCase{
		{
			description: "allow filters",
			input:       []string{"pod", "monitor"},
			expected: LogFilters{
				allow: []LogFilter{
					{spanPrefix: "pod", not: false},
					{spanPrefix: "monitor", not: false},
				},
			},
		},
		{
			description: "deny filters",
			input:       []string{"!pod", "!monitor"},
			expected: LogFilters{
				deny: []LogFilter{
					{spanPrefix: "pod", not: true},
					{spanPrefix: "monitor", not: true},
				},
			},
		},
		{
			description: "allow and deny filters",
			input:       []string{"!pod", "!monitor", "build"},
			expected: LogFilters{
				allow: []LogFilter{
					{spanPrefix: "build", not: false},
				},
				deny: []LogFilter{
					{spanPrefix: "pod", not: true},
					{spanPrefix: "monitor", not: true},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			actual := LogFiltersFromStrings(tc.input)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

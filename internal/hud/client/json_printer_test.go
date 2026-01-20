package client

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

func TestJSONPrinterAllFields(t *testing.T) {
	buf := &bytes.Buffer{}
	printer := NewJSONPrinter(Stdout(buf))

	testTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	lines := []logstore.LogLine{
		{
			Text:         "Build started\n",
			SpanID:       "build:1",
			ManifestName: "api",
			Level:        logger.InfoLvl,
			Time:         testTime,
			ProgressID:   "",
			BuildEvent:   "",
		},
	}

	printer.Print(lines)

	output := buf.String()
	assert.True(t, strings.HasSuffix(output, "\n"), "JSONL should end with newline")

	var result map[string]interface{}
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	// All fields should be present
	assert.Equal(t, "2025-01-15T10:30:00Z", result["time"])
	assert.Equal(t, "api", result["resource"])
	assert.Equal(t, "info", result["level"])
	assert.Equal(t, "Build started", result["message"])
	assert.Equal(t, "build:1", result["spanID"])
	assert.Equal(t, "", result["progressID"])
	assert.Equal(t, "", result["buildEvent"])
	assert.Equal(t, "build", result["source"])
}

func TestJSONPrinterMultipleLines(t *testing.T) {
	buf := &bytes.Buffer{}
	printer := NewJSONPrinter(Stdout(buf))

	testTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	lines := []logstore.LogLine{
		{Text: "Line 1\n", ManifestName: "api", Level: logger.InfoLvl, Time: testTime},
		{Text: "Line 2\n", ManifestName: "api", Level: logger.WarnLvl, Time: testTime.Add(time.Second)},
		{Text: "Line 3\n", ManifestName: "api", Level: logger.ErrorLvl, Time: testTime.Add(2 * time.Second)},
	}

	printer.Print(lines)

	output := buf.String()
	outputLines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	assert.Len(t, outputLines, 3, "should have 3 JSON lines")

	// Each line should be valid JSON
	for i, line := range outputLines {
		var result map[string]interface{}
		err := json.Unmarshal([]byte(line), &result)
		require.NoError(t, err, "line %d should be valid JSON", i)
	}
}

func TestJSONPrinterSourceField(t *testing.T) {
	testCases := []struct {
		spanID         string
		expectedSource string
	}{
		{"build:1", "build"},
		{"cmdimage:nginx", "build"},
		{"pod:default:nginx", "runtime"},
		{"tiltfile:(Tiltfile):1", "runtime"},
	}

	for _, tc := range testCases {
		t.Run(tc.spanID, func(t *testing.T) {
			buf := &bytes.Buffer{}
			printer := NewJSONPrinter(Stdout(buf))

			lines := []logstore.LogLine{
				{Text: "test\n", SpanID: logstore.SpanID(tc.spanID), Time: time.Now()},
			}

			printer.Print(lines)

			var result map[string]interface{}
			err := json.Unmarshal(buf.Bytes(), &result)
			require.NoError(t, err)

			assert.Equal(t, tc.expectedSource, result["source"])
		})
	}
}

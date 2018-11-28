package hud

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type durationCase struct {
	dur    time.Duration
	deploy string
	build  string
}

func TestFormatDurations(t *testing.T) {
	table := []durationCase{
		{time.Second, "<5s", "1.00s"},
		{10 * time.Second, "<15s", "10.00s"},
		{20 * time.Second, "<30s", "20s"},
		{40 * time.Second, "<45s", "40s"},
		{50 * time.Second, "<1m", "50s"},
		{70 * time.Second, "1m", "1m"},
		{150 * time.Second, "2m", "2m"},
		{4000 * time.Second, "1h", "1h"},
	}

	for i, entry := range table {
		t.Run(fmt.Sprintf("Case%d", i), func(t *testing.T) {
			assert.Equal(t, entry.build, formatBuildDuration(entry.dur), "formatBuildDuration")
			assert.Equal(t, entry.deploy, formatDeployAge(entry.dur), "formatDeployAge")
		})
	}
}

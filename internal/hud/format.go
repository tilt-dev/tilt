package hud

import (
	"fmt"
	"math"
	"time"
)

// Duplicated in web/src/format.ts
func formatBuildDuration(d time.Duration) string {
	hours := int(d.Hours())
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}

	minutes := int(d.Minutes())
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}

	seconds := d.Seconds()
	if seconds >= 9.95 {
		return fmt.Sprintf("%ds", int(math.Round(seconds)))
	}

	fractionalSeconds := float64(d) / float64(time.Second)
	return fmt.Sprintf("%0.1fs", fractionalSeconds)
}

func formatDeployAge(d time.Duration) string {
	switch {
	case d.Seconds() < 5:
		return "<5s"
	case d.Seconds() < 15:
		return "<15s"
	case d.Seconds() < 30:
		return "<30s"
	case d.Seconds() < 45:
		return "<45s"
	case d.Minutes() < 1:
		return "<1m"
	case d.Hours() < 1:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
}

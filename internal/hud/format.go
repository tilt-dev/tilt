package hud

import (
	"fmt"
	"strings"
	"time"
)

func formatBuildDuration(d time.Duration) string {
	hours := int(d.Hours())
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}

	minutes := int(d.Minutes())
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}

	seconds := int(d.Seconds())
	if seconds > 10 {
		return fmt.Sprintf("%ds", seconds)
	}

	fractionalSeconds := float64(d) / float64(time.Second)
	return fmt.Sprintf("%0.2fs", fractionalSeconds)
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

func formatFileList(files []string) string {
	const maxFilesToDisplay = 3

	var ret []string

	for i, f := range files {
		if i > maxFilesToDisplay {
			ret = append(ret, fmt.Sprintf("(%d more)", len(files)-maxFilesToDisplay))
			break
		}
		ret = append(ret, f)
	}

	return strings.Join(ret, ", ")
}

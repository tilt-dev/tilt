package hud

import (
	"strings"
	"time"

	"github.com/gdamore/tcell"

	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/rty"
)

// The most lines we can reasonably put in the log pane. If the log pane sticks
// around in the long term, we might want to compute this dynamically based on
// the window size.
const logLineCount = view.LogLineCount

func deployTimeText(t time.Time) rty.Component {
	sb := rty.NewStringBuilder()
	if t.IsZero() {
		sb.Text("-")
	} else {
		sb.Textf("%s ago", formatDeployAge(time.Since(t)))
	}
	return sb.Build()
}

func deployTimeCell(t time.Time, color tcell.Color) rty.Component {
	return rty.NewMinLengthLayout(DeployCellMinWidth, rty.DirHor).
		SetAlign(rty.AlignEnd).
		Add(rty.Fg(deployTimeText(t), color))
}

func middotText() rty.Component {
	return rty.ColoredString(" • ", cLightText)
}

const abbreviatedLogLineCount = 6

func abbreviateLog(s string) []string {
	lines := strings.Split(s, "\n")
	start := len(lines) - abbreviatedLogLineCount
	if start < 0 {
		start = 0
	}

	// skip past leading empty lines
	for {
		if start < len(lines) && len(strings.TrimSpace(lines[start])) == 0 {
			start++
		} else {
			break
		}
	}

	return lines[start:]
}

package hud

import (
	"strings"
	"time"

	"github.com/windmilleng/tilt/internal/rty"
)

func deployTimeText(t time.Time) rty.Component {
	sb := rty.NewStringBuilder()
	if t.IsZero() {
		sb.Text("-")
	} else {
		sb.Textf("%s ago", formatDeployAge(time.Since(t)))
	}
	return sb.Build()
}

func deployTimeCell(t time.Time) rty.Component {
	return rty.NewMinLengthLayout(DeployCellMinWidth, rty.DirHor).
		SetAlign(rty.AlignEnd).
		Add(deployTimeText(t))
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

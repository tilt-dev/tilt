package hud

import (
	"io"
	"time"

	"github.com/windmilleng/tilt/pkg/model/logstore"
)

var backoffInit = 5 * time.Second
var backoffMultiplier = time.Duration(2)

type Stdout io.Writer

type IncrementalPrinter struct {
	progress map[progressKey]progressStatus
	stdout   Stdout
}

func NewIncrementalPrinter(stdout Stdout) *IncrementalPrinter {
	return &IncrementalPrinter{
		progress: make(map[progressKey]progressStatus),
		stdout:   stdout,
	}
}

func (p *IncrementalPrinter) Print(lines []logstore.LogLine) {
	for _, line := range lines {
		// Naive progress implementation: skip lines that have already been printed
		// recently. This works with any output stream.
		//
		// TODO(nick): Use ANSI codes to overwrite previous lines. It requires
		// a little extra bookkeeping about where to find the progress line,
		// and only works on terminals.
		progressID := line.ProgressID
		key := progressKey{spanID: line.SpanID, progressID: progressID}
		if progressID != "" {
			status, hasBeenPrinted := p.progress[key]
			shouldPrint := line.ProgressMustPrint ||
				!hasBeenPrinted ||
				line.Time.Sub(status.lastPrinted) > status.printWait
			if !shouldPrint {
				continue
			}
		}
		_, _ = io.WriteString(p.stdout, line.Text)

		if progressID != "" {
			status := p.progress[key]
			newWait := backoffInit
			if status.printWait > 0 {
				newWait = backoffMultiplier * status.printWait
			}
			p.progress[key] = progressStatus{
				lastPrinted: line.Time,
				printWait:   newWait,
			}
		}
	}
}

type progressKey struct {
	spanID     logstore.SpanID
	progressID string
}

type progressStatus struct {
	lastPrinted time.Time
	printWait   time.Duration
}

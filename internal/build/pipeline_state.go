package build

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/windmilleng/tilt/internal/logger"
)

type PipelineState struct {
	curPipelineStep        int
	curBuildStep           int
	totalPipelineStepCount int
	pipelineStepDurations  []time.Duration
	curPipelineStart       time.Time
	curPipelineStepStart   time.Time
}

const buildStepOutputPrefix = "    ╎ "

func NewPipelineState(ctx context.Context, totalStepCount int) *PipelineState {
	l := logger.Get(ctx)
	l.Infof("%s", logger.Blue(l).Sprint("──┤ Pipeline Starting… ├──────────────────────────────────────────────"))

	return &PipelineState{
		curPipelineStep:        1,
		totalPipelineStepCount: totalStepCount,
		curPipelineStart:       time.Now(),
	}
}

// NOTE(maia): this func should always be deferred in a closure, so that the `err` arg
// is bound at the time of calling rather than at the time of deferring. I.e., do:
//     defer func() { ps.End(err) }()
// and NOT:
//     defer ps.End(err)
func (ps *PipelineState) End(ctx context.Context, err error) {
	l := logger.Get(ctx)

	elapsed := time.Now().Sub(ps.curPipelineStart)

	if err != nil {
		prefix := logger.Red(l).Sprint(" ︎ERROR:")
		l.Infof("%s %s\n", prefix, err.Error())
		ps.curPipelineStep = 0
		ps.curBuildStep = 0
		return
	}

	for i, duration := range ps.pipelineStepDurations {
		l.Infof("  │ Step %d - %.3fs │", i+1, duration.Seconds())
	}

	time := logger.Green(l).Sprintf("%.3fs", elapsed.Seconds())
	l.Infof("──┤ Done in: %s ︎├──\n", time)
	ps.curPipelineStep = 0
	ps.curBuildStep = 0
}

func (ps *PipelineState) StartPipelineStep(ctx context.Context, format string, a ...interface{}) {
	l := logger.Get(ctx)
	line := logger.Green(l).Sprintf("STEP %d/%d — %s", ps.curPipelineStep, ps.totalPipelineStepCount, fmt.Sprintf(format, a...))
	l.Infof("%s", line)
	ps.curPipelineStep++
	ps.curBuildStep = 1
	ps.curPipelineStepStart = time.Now()
}

func (ps *PipelineState) EndPipelineStep(ctx context.Context) {
	l := logger.Get(ctx)
	elapsed := time.Now().Sub(ps.curPipelineStepStart)
	l.Infof("    (Done %.3fs)\n", elapsed.Seconds())
	ps.pipelineStepDurations = append(ps.pipelineStepDurations, elapsed)
}

func (ps *PipelineState) StartBuildStep(ctx context.Context, format string, a ...interface{}) {
	l := logger.Get(ctx)
	l.Infof("  → %s", fmt.Sprintf(format, a...))
	ps.curBuildStep++
}

func (ps *PipelineState) Printf(ctx context.Context, format string, a ...interface{}) {
	l := logger.Get(ctx)
	if ps.curBuildStep == 0 {
		l.Infof(format, a...)
	} else {
		message := fmt.Sprintf(format, a...)
		message = strings.Replace(message, "\n", "\n"+buildStepOutputPrefix, -1)
		l.Infof("%s%s", buildStepOutputPrefix, message)
	}
}

func (ps *PipelineState) Writer(ctx context.Context) io.Writer {
	l := logger.Get(ctx)
	underlying := l.Writer(logger.InfoLvl)
	if ps.curBuildStep == 0 {
		return underlying
	} else {
		return logger.NewPrefixedWriter(buildStepOutputPrefix, underlying)
	}
}

package build

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/windmilleng/tilt/pkg/logger"
)

type PipelineState struct {
	curPipelineStep        int
	curBuildStep           int
	totalPipelineStepCount int
	pipelineStepDurations  []time.Duration
	curPipelineStart       time.Time
	curPipelineStepStart   time.Time
	c                      Clock
}

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func ProvideClock() Clock {
	return realClock{}
}

const buildStepOutputPrefix = "     "

func NewPipelineState(ctx context.Context, totalStepCount int, c Clock) *PipelineState {
	return &PipelineState{
		curPipelineStep:        1,
		totalPipelineStepCount: totalStepCount,
		curPipelineStart:       c.Now(),
		c:                      c,
	}
}

// NOTE(maia): this func should always be deferred in a closure, so that the `err` arg
// is bound at the time of calling rather than at the time of deferring. I.e., do:
//     defer func() { ps.End(ctx, err) }()
// and NOT:
//     defer ps.End(ctx, err)
func (ps *PipelineState) End(ctx context.Context, err error) {
	ps.curPipelineStep = 0
	ps.curBuildStep = 0

	if err != nil {
		return
	}

	l := logger.Get(ctx)

	elapsed := ps.c.Now().Sub(ps.curPipelineStart)

	for i, duration := range ps.pipelineStepDurations {
		l.Infof("%sStep %d - %.2fs", buildStepOutputPrefix, i+1, duration.Seconds())
	}

	t := logger.Blue(l).Sprintf("%.2fs", elapsed.Seconds())
	l.Infof("%sDONE IN: %s \n", buildStepOutputPrefix, t)
}

func (ps *PipelineState) StartPipelineStep(ctx context.Context, format string, a ...interface{}) {
	l := logger.Get(ctx)
	line := logger.Blue(l).Sprintf("STEP %d/%d", ps.curPipelineStep, ps.totalPipelineStepCount)
	l.Infof("%s — %s", line, fmt.Sprintf(format, a...))
	ps.curPipelineStep++
	ps.curBuildStep = 1
	ps.curPipelineStepStart = ps.c.Now()
}

func (ps *PipelineState) EndPipelineStep(ctx context.Context) {
	elapsed := ps.c.Now().Sub(ps.curPipelineStepStart)
	logger.Get(ctx).Infof("")
	ps.pipelineStepDurations = append(ps.pipelineStepDurations, elapsed)
}

func (ps *PipelineState) StartBuildStep(ctx context.Context, format string, a ...interface{}) {
	l := logger.Get(ctx)
	l.Infof("%s%s", buildStepOutputPrefix, fmt.Sprintf(format, a...))
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

func (ps *PipelineState) AttachLogger(ctx context.Context) context.Context {
	l := logger.Get(ctx)
	if ps.curBuildStep == 0 {
		return ctx
	} else {
		return logger.WithLogger(ctx,
			logger.NewPrefixedLogger(buildStepOutputPrefix, l))
	}
}

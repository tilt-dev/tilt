package build

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tilt-dev/tilt/pkg/logger"
)

type PipelineState struct {
	curPipelineStep        PipelineStep
	curBuildStep           int
	totalPipelineStepCount int
	pipelineSteps          []PipelineStep
	curPipelineStart       time.Time
	curPipelineStepStart   time.Time
	c                      Clock
}

type PipelineStep struct {
	Name     string        // for logging
	Index    int           // human-readable, 1-indexed
	Duration time.Duration // not populated until end of the step
}

func (pStep PipelineStep) withDuration(d time.Duration) PipelineStep {
	pStep.Duration = d
	return pStep
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
		curPipelineStep:        PipelineStep{Index: 0},
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
	ps.curPipelineStep = PipelineStep{Index: 0}
	ps.curBuildStep = 0

	if err != nil {
		return
	}

	l := logger.Get(ctx)

	elapsed := ps.c.Now().Sub(ps.curPipelineStart)

	for i, step := range ps.pipelineSteps {
		l.Infof("%sStep %d - %.2fs (%s)", buildStepOutputPrefix, i+1, step.Duration.Seconds(), step.Name)
	}

	t := logger.Blue(l).Sprintf("%.2fs", elapsed.Seconds())
	l.Infof("%sDONE IN: %s \n", buildStepOutputPrefix, t)
}

func (ps *PipelineState) curPipelineIndex() int {
	return ps.curPipelineStep.Index
}

func (ps *PipelineState) StartPipelineStep(ctx context.Context, format string, a ...interface{}) {
	l := logger.Get(ctx)
	stepName := fmt.Sprintf(format, a...)
	ps.curPipelineStep = PipelineStep{
		Name:  stepName,
		Index: ps.curPipelineIndex() + 1,
	}
	line := logger.Blue(l).Sprintf("STEP %d/%d", ps.curPipelineIndex(), ps.totalPipelineStepCount)
	l.Infof("%s — %s", line, stepName)
	ps.curBuildStep = 1
	ps.curPipelineStepStart = ps.c.Now()
}

func (ps *PipelineState) EndPipelineStep(ctx context.Context) {
	elapsed := ps.c.Now().Sub(ps.curPipelineStepStart)
	logger.Get(ctx).Infof("")
	ps.pipelineSteps = append(ps.pipelineSteps, ps.curPipelineStep.withDuration(elapsed))
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

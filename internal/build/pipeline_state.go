package build

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tilt-dev/tilt/pkg/logger"
)

type PipelineState struct {
	totalPipelineStepCount int
	curBuildStep           int
	curPipelineStart       time.Time
	pipelineSteps          []PipelineStep
	c                      Clock
}

type PipelineStep struct {
	Name      string // for logging
	StartTime time.Time
	Duration  time.Duration // not populated until end of the step
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
		totalPipelineStepCount: totalStepCount,
		pipelineSteps:          []PipelineStep{},
		curPipelineStart:       c.Now(),
		c:                      c,
	}
}

// NOTE(maia): this func should always be deferred in a closure, so that the `err` arg
// is bound at the time of calling rather than at the time of deferring. I.e., do:
//
//	defer func() { ps.End(ctx, err) }()
//
// and NOT:
//
//	defer ps.End(ctx, err)
func (ps *PipelineState) End(ctx context.Context, err error) {
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
	// human-readable i.e. 1-indexed
	return len(ps.pipelineSteps)
}

func (ps *PipelineState) curPipelineStep() PipelineStep {
	if len(ps.pipelineSteps) == 0 {
		return PipelineStep{}
	}
	return ps.pipelineSteps[len(ps.pipelineSteps)-1]
}

func (ps *PipelineState) StartPipelineStep(ctx context.Context, format string, a ...interface{}) {
	l := logger.Get(ctx)
	stepName := fmt.Sprintf(format, a...)
	ps.pipelineSteps = append(ps.pipelineSteps, PipelineStep{
		Name:      stepName,
		StartTime: ps.c.Now(),
	})
	line := logger.Blue(l).Sprintf("STEP %d/%d", ps.curPipelineIndex(), ps.totalPipelineStepCount)
	l.Infof("%s — %s", line, stepName)
	ps.curBuildStep = 1
}

func (ps *PipelineState) EndPipelineStep(ctx context.Context) {
	elapsed := ps.c.Now().Sub(ps.curPipelineStep().StartTime)
	logger.Get(ctx).Infof("")
	ps.pipelineSteps[len(ps.pipelineSteps)-1].Duration = elapsed
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
		message = strings.ReplaceAll(message, "\n", "\n"+buildStepOutputPrefix)
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

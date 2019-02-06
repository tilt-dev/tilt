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

const buildStepOutputPrefix = "    ╎ "

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
//     defer func() { ps.End(err) }()
// and NOT:
//     defer ps.End(err)
func (ps *PipelineState) End(ctx context.Context, err error) {
	l := logger.Get(ctx)
	prefix := logger.Blue(l).Sprint("  │ ")

	elapsed := ps.c.Now().Sub(ps.curPipelineStart)

	if err != nil {
		prefix := logger.Red(l).Sprint("ERROR:")
		l.Infof("%s %s", prefix, err.Error())
		ps.curPipelineStep = 0
		ps.curBuildStep = 0
		return
	}

	for i, duration := range ps.pipelineStepDurations {
		l.Infof("%sStep %d - %.3fs", prefix, i+1, duration.Seconds())
	}

	l.Infof("%sDone in: %.3fs \n", prefix, elapsed.Seconds())
	ps.curPipelineStep = 0
	ps.curBuildStep = 0
}

func (ps *PipelineState) StartPipelineStep(ctx context.Context, format string, a ...interface{}) {
	l := logger.Get(ctx)
	line := logger.Blue(l).Sprintf("STEP %d/%d — ", ps.curPipelineStep, ps.totalPipelineStepCount)
	l.Infof("%s%s", line, fmt.Sprintf(format, a...))
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
	p := logger.Blue(l).Sprint("  │ ")
	l.Infof("%s%s", p, fmt.Sprintf(format, a...))
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

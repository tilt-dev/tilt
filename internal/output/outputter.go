package output

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/windmilleng/tilt/internal/logger"
)

type Color string

const (
	cGreen                = Color("\033[32m")
	cBlue                 = Color("\033[34m")
	cReset                = Color("\u001b[0m")
	buildStepOutputPrefix = "    ╎ "
)

const outputterContextKey = "outputter"

type Outputter struct {
	logger                logger.Logger
	indentation           int
	curBuildStep          int
	curPipelineStep       int
	pipelineStepDurations []time.Duration
	curPipelineStart      time.Time
	curPipelineStepStart  time.Time
}

func Get(ctx context.Context) *Outputter {
	val := ctx.Value(outputterContextKey)

	if val != nil {
		return val.(*Outputter)
	}

	// No outputter found in context, something is wrong.
	panic("Called output.Get(ctx) on a context with no Outputter attached!")
}

func NewOutputter(logger logger.Logger) Outputter {
	return Outputter{logger: logger}
}

func WithOutputter(ctx context.Context, outputter Outputter) context.Context {
	return context.WithValue(ctx, outputterContextKey, &outputter)
}

func (o *Outputter) printColorf(color Color, format string, a ...interface{}) {
	o.logger.Infof("%s%s%s", string(color), fmt.Sprintf(format, a...), cReset)
}

func (o *Outputter) StartPipeline() {
	o.printColorf(cBlue, "──┤ Pipeline Starting … ├────────────────────────────────────────")
	o.curPipelineStep = 1
	o.pipelineStepDurations = nil
	o.curPipelineStart = time.Now()
}

func (o *Outputter) EndPipeline() {
	for i, duration := range o.pipelineStepDurations {
		o.logger.Infof("  │ Step %d - %.3fs", i+1, duration.Seconds())
	}

	elapsed := time.Now().Sub(o.curPipelineStart)
	o.logger.Infof("%s──┤ ︎Pipeline Done in %s%.3fs%s ⚡︎├────────────────────────────────────%s",
		cBlue,
		cGreen,
		elapsed.Seconds(),
		cBlue,
		cReset)
	o.curPipelineStep = 0
}

// how many steps there are per pipeline
// we might need to change or remove this if it turns out we can't predict how many steps there will be
const numSteps = 2

func (o *Outputter) StartPipelineStep(format string, a ...interface{}) {
	o.printColorf(cGreen, "STEP %d/%d — %s", o.curPipelineStep, numSteps, fmt.Sprintf(format, a...))
	o.curPipelineStep++
	o.curBuildStep = 1
	o.curPipelineStepStart = time.Now()
}

func (o *Outputter) EndPipelineStep() {
	elapsed := time.Now().Sub(o.curPipelineStepStart)
	o.logger.Infof("    (Done %.3fs)", elapsed.Seconds())
	o.logger.Infof("")
	o.pipelineStepDurations = append(o.pipelineStepDurations, elapsed)
}

func (o *Outputter) StartBuildStep(format string, a ...interface{}) {
	o.logger.Infof("  → %s", fmt.Sprintf(format, a...))
	o.curBuildStep++
}

func (o *Outputter) Print(format string, a ...interface{}) {
	if o.curBuildStep == 0 {
		o.logger.Infof(format, a)
	} else {
		o.logger.Infof("%s%s", buildStepOutputPrefix, fmt.Sprintf(format, a...))
	}
}

// sticks "prefix" at the start of every new line
type prefixedWriter struct {
	prefix                string
	underlying            io.Writer
	indentBeforeNextWrite bool
}

var _ io.Writer = &prefixedWriter{}

func newPrefixedWriter(prefix string, underlying io.Writer) *prefixedWriter {
	return &prefixedWriter{prefix, underlying, true}
}

func (i *prefixedWriter) Write(buf []byte) (n int, err error) {
	for _, c := range buf {
		if i.indentBeforeNextWrite {
			_, err := i.underlying.Write([]byte(i.prefix))
			if err != nil {
				return n, err
			}
		}
		nn, err := i.underlying.Write([]byte{c})
		n += nn
		if err != nil {
			return n, err
		}
		i.indentBeforeNextWrite = c == '\n'
	}
	return len(buf), nil
}

func (o Outputter) Writer() io.Writer {
	underlying := o.logger.Writer(logger.InfoLvl)
	if o.curBuildStep == 0 {
		return underlying
	} else {
		return newPrefixedWriter(buildStepOutputPrefix, underlying)
	}
}

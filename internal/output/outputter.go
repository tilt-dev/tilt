package output

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/windmilleng/tilt/internal/logger"
)

const (
	buildStepOutputPrefix = "    ╎ "
	outputterContextKey   = "outputter"
)

type Outputter struct {
	logger                 logger.Logger
	indentation            int
	curBuildStep           int
	curPipelineStep        int
	totalPipelineStepCount int
	pipelineStepDurations  []time.Duration
	curPipelineStart       time.Time
	curPipelineStepStart   time.Time
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

func (o *Outputter) printColorf(color *color.Color, format string, a ...interface{}) {
	o.logger.Infof(color.Sprintf(format, a...))
}

func (o *Outputter) StartPipeline(totalStepCount int) {
	o.printColorf(color.New(color.FgBlue), "──┤ Pipeline Starting … ├────────────────────────────────────────")
	o.curPipelineStep = 1
	o.totalPipelineStepCount = totalStepCount
	o.pipelineStepDurations = nil
	o.curPipelineStart = time.Now()
}

func (o *Outputter) EndPipeline() {
	for i, duration := range o.pipelineStepDurations {
		o.logger.Infof("  │ Step %d - %.3fs", i+1, duration.Seconds())
	}

	elapsed := time.Now().Sub(o.curPipelineStart)

	blue := color.New(color.FgBlue).SprintfFunc()
	green := color.New(color.FgGreen).SprintfFunc()

	o.logger.Infof(blue("──┤ ︎Pipeline Done in %s ⚡︎├───────────────────────────────────", green("%.3fs", elapsed.Seconds())))
	o.curPipelineStep = 0
}

func (o *Outputter) StartPipelineStep(format string, a ...interface{}) {
	o.printColorf(color.New(color.FgGreen), "STEP %d/%d — %s", o.curPipelineStep, o.totalPipelineStepCount, fmt.Sprintf(format, a...))
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
	output := ""

	if i.indentBeforeNextWrite {
		output += i.prefix
	}

	output += string(buf)

	// temporarily take off a trailing newline so that Replace doesn't add a prefix at the end
	endsInNewline := output[len(output)-1] == '\n'
	if endsInNewline {
		output = output[:len(output)-1]
	}

	output = strings.Replace(output, "\n", "\n"+i.prefix, -1)

	if endsInNewline {
		output = output + "\n"
		i.indentBeforeNextWrite = true
	} else {
		i.indentBeforeNextWrite = false
	}

	_, err = i.underlying.Write([]byte(output))
	if err != nil {
		return 0, err
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

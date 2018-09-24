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
	logger logger.Logger

	curPipelineStep        int
	curBuildStep           int
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
	return Outputter{
		logger: logger,
	}
}

func WithOutputter(ctx context.Context, outputter Outputter) context.Context {
	return context.WithValue(ctx, outputterContextKey, &outputter)
}

func (o *Outputter) color(c color.Attribute) *color.Color {
	color := color.New(c)
	if !o.logger.SupportsColor() {
		color.DisableColor()
	}
	return color
}

func (o *Outputter) blue() *color.Color   { return o.color(color.FgBlue) }
func (o *Outputter) yellow() *color.Color { return o.color(color.FgYellow) }
func (o *Outputter) green() *color.Color  { return o.color(color.FgGreen) }
func (o *Outputter) Red() *color.Color    { return o.color(color.FgRed) }

func (o *Outputter) StartPipeline(totalStepCount int) {
	o.logger.Infof("%s", o.blue().Sprint("──┤ Pipeline Starting… ├──────────────────────────────────────────────"))
	o.curPipelineStep = 1
	o.totalPipelineStepCount = totalStepCount
	o.pipelineStepDurations = nil
	o.curPipelineStart = time.Now()
}

// NOTE(maia): this func should always be deferred in a closure, so that the `err` arg
// is bound at the time of calling rather than at the time of deferring. I.e., do:
//     defer func() { o.EndPipeline(err) }()
// and NOT:
//     defer o.EndPipeline(err)
func (o *Outputter) EndPipeline(err error) {
	elapsed := time.Now().Sub(o.curPipelineStart)

	if err != nil {
		prefix := o.Red().Sprint(" ︎ERROR:")
		o.logger.Infof("%s %s\n", prefix, err.Error())
		o.curPipelineStep = 0
		o.curBuildStep = 0
		return
	}

	for i, duration := range o.pipelineStepDurations {
		o.logger.Infof("  │ Step %d - %.3fs │", i+1, duration.Seconds())
	}

	time := o.green().Sprintf("%.3fs", elapsed.Seconds())
	o.logger.Infof("──┤ Done in: %s ︎├──\n", time)
	o.curPipelineStep = 0
	o.curBuildStep = 0
}

func (o *Outputter) StartPipelineStep(format string, a ...interface{}) {
	line := o.green().Sprintf("STEP %d/%d — %s", o.curPipelineStep, o.totalPipelineStepCount, fmt.Sprintf(format, a...))
	o.logger.Infof("%s", line)
	o.curPipelineStep++
	o.curBuildStep = 1
	o.curPipelineStepStart = time.Now()
}

func (o *Outputter) EndPipelineStep() {
	elapsed := time.Now().Sub(o.curPipelineStepStart)
	o.logger.Infof("    (Done %.3fs)\n", elapsed.Seconds())
	o.pipelineStepDurations = append(o.pipelineStepDurations, elapsed)
}

func (o *Outputter) StartBuildStep(format string, a ...interface{}) {
	o.logger.Infof("  → %s", fmt.Sprintf(format, a...))
	o.curBuildStep++
}

func (o *Outputter) Summary(format string, a ...interface{}) {
	o.logger.Infof("%s", o.blue().Sprint("──┤ Status ├──────────────────────────────────────────────────────────"))
	o.logger.Infof(format, a...)
	o.logger.Infof("%s", o.blue().Sprint("╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴╴"))
}

func (o *Outputter) PrintColorf(color *color.Color, format string, a ...interface{}) {
	o.Printf(color.Sprintf(format, a...))
}

func (o *Outputter) Printf(format string, a ...interface{}) {
	if o.curBuildStep == 0 {
		o.logger.Infof(format, a...)
	} else {
		message := fmt.Sprintf(format, a...)
		message = strings.Replace(message, "\n", "\n"+buildStepOutputPrefix, -1)
		o.logger.Infof("%s%s", buildStepOutputPrefix, message)
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
	endsInNewline := false
	if len(output) > 0 {
		endsInNewline = output[len(output)-1] == '\n'
	}

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

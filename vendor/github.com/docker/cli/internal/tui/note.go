// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.23

package tui

import (
	"fmt"
	"strings"

	"github.com/morikuni/aec"
)

var InfoHeader = Str{
	Plain: " Info -> ",
	Fancy: aec.Bold.Apply(aec.LightCyanB.Apply(aec.BlackF.Apply("i")) + " " + aec.LightCyanF.Apply("Info → ")),
}

type options struct {
	header Str
}

type noteOptions func(o *options)

func withHeader(header Str) noteOptions {
	return func(o *options) {
		o.header = header
	}
}

func (o Output) printNoteWithOptions(format string, args []any, opts ...noteOptions) {
	if o.isTerminal {
		// TODO: Handle all flags
		format = strings.ReplaceAll(format, "--platform", ColorFlag.Apply("--platform"))
	}

	opt := &options{
		header: InfoHeader,
	}

	for _, override := range opts {
		override(opt)
	}

	h := o.Sprint(opt.header)

	_, _ = fmt.Fprint(o, "\n", h)
	s := fmt.Sprintf(format, args...)
	for idx, line := range strings.Split(s, "\n") {
		if idx > 0 {
			_, _ = fmt.Fprint(o, strings.Repeat(" ", Width(h)))
		}

		l := line
		if o.isTerminal {
			l = aec.Italic.Apply(l)
		}
		_, _ = fmt.Fprintln(o, l)
	}
}

func (o Output) PrintNote(format string, args ...any) {
	o.printNoteWithOptions(format, args, withHeader(InfoHeader))
}

var warningHeader = Str{
	Plain: " Warn -> ",
	Fancy: aec.Bold.Apply(aec.LightYellowB.Apply(aec.BlackF.Apply("w")) + " " + ColorWarning.Apply("Warn → ")),
}

func (o Output) PrintWarning(format string, args ...any) {
	o.printNoteWithOptions(format, args, withHeader(warningHeader))
}

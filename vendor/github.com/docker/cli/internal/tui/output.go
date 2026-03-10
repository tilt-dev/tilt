// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.24

package tui

import (
	"fmt"
	"os"

	"github.com/docker/cli/cli/streams"
	"github.com/morikuni/aec"
)

type Output struct {
	*streams.Out
	noColor bool
}

type terminalPrintable interface {
	String(isTerminal bool) string
}

func NewOutput(out *streams.Out) Output {
	noColor := !out.IsTerminal()
	if os.Getenv("NO_COLOR") != "" {
		noColor = true
	}
	return Output{
		Out:     out,
		noColor: noColor,
	}
}

func (o Output) Color(clr aec.ANSI) aec.ANSI {
	if o.noColor {
		return ColorNone
	}
	return clr
}

func (o Output) Sprint(all ...any) string {
	var out []any
	for _, p := range all {
		if s, ok := p.(terminalPrintable); ok {
			out = append(out, s.String(!o.noColor))
		} else {
			out = append(out, p)
		}
	}
	return fmt.Sprint(out...)
}

func (o Output) PrintlnWithColor(clr aec.ANSI, args ...any) {
	msg := o.Sprint(args...)
	if !o.noColor {
		msg = clr.Apply(msg)
	}
	_, _ = fmt.Fprintln(o.Out, msg)
}

func (o Output) Println(p ...any) {
	_, _ = fmt.Fprintln(o.Out, o.Sprint(p...))
}

func (o Output) Print(p ...any) {
	_, _ = fmt.Print(o.Out, o.Sprint(p...))
}

// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.23

package tui

import (
	"github.com/morikuni/aec"
)

var (
	ColorTitle     = aec.NewBuilder(aec.DefaultF, aec.Bold).ANSI
	ColorPrimary   = aec.NewBuilder(aec.DefaultF, aec.Bold).ANSI
	ColorSecondary = aec.DefaultF
	ColorTertiary  = aec.NewBuilder(aec.DefaultF, aec.Faint).ANSI
	ColorLink      = aec.NewBuilder(aec.LightCyanF, aec.Underline).ANSI
	ColorWarning   = aec.LightYellowF
	ColorFlag      = aec.NewBuilder(aec.Bold).ANSI
	ColorNone      = aec.ANSI(noColor{})
)

type noColor struct{}

func (a noColor) With(_ ...aec.ANSI) aec.ANSI {
	return a
}

func (noColor) Apply(s string) string {
	return s
}

func (noColor) String() string {
	return ""
}

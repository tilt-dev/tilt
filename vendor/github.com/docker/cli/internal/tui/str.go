// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.23

package tui

type Str struct {
	// Fancy is the fancy string representation of the string.
	Fancy string

	// Plain is the plain string representation of the string.
	Plain string
}

func (p Str) String(isTerminal bool) string {
	if isTerminal {
		return p.Fancy
	}
	return p.Plain
}

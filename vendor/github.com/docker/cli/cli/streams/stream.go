package streams

import (
	"github.com/moby/term"
)

type commonStream struct {
	fd         uintptr
	isTerminal bool
	state      *term.State
}

// FD returns the file descriptor number for this stream.
func (s *commonStream) FD() uintptr {
	return s.fd
}

// IsTerminal returns true if this stream is connected to a terminal.
func (s *commonStream) IsTerminal() bool {
	return s.isTerminal
}

// RestoreTerminal restores normal mode to the terminal.
func (s *commonStream) RestoreTerminal() {
	if s.state != nil {
		_ = term.RestoreTerminal(s.fd, s.state)
	}
}

// SetIsTerminal overrides whether a terminal is connected. It is used to
// override this property in unit-tests, and should not be depended on for
// other purposes.
func (s *commonStream) SetIsTerminal(isTerminal bool) {
	s.isTerminal = isTerminal
}

// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.25

package streams

import (
	"os"

	"github.com/moby/term"
	"github.com/sirupsen/logrus"
)

func newCommonStream(stream any) commonStream {
	var f *os.File
	if v, ok := stream.(*os.File); ok {
		f = v
	}

	fd, tty := term.GetFdInfo(stream)
	return commonStream{
		f:   f,
		fd:  fd,
		tty: tty,
	}
}

type commonStream struct {
	f     *os.File
	fd    uintptr
	tty   bool
	state *term.State
}

// FD returns the file descriptor number for this stream.
func (s *commonStream) FD() uintptr { return s.fd }

// file returns the underlying *os.File if the stream was constructed from one.
func (s *commonStream) file() (*os.File, bool) { return s.f, s.f != nil }

// isTerminal returns whether this stream is connected to a terminal.
func (s *commonStream) isTerminal() bool { return s.tty }

// setIsTerminal overrides whether a terminal is connected for testing.
func (s *commonStream) setIsTerminal(isTerminal bool) { s.tty = isTerminal }

// restoreTerminal restores the terminal state if SetRawTerminal succeeded earlier.
func (s *commonStream) restoreTerminal() {
	if s.state != nil {
		_ = term.RestoreTerminal(s.fd, s.state)
	}
}

func (s *commonStream) setRawTerminal(setter func(uintptr) (*term.State, error)) error {
	if !s.tty || os.Getenv("NORAW") != "" {
		return nil
	}
	state, err := setter(s.fd)
	if err != nil {
		return err
	}
	s.state = state
	return nil
}

func (s *commonStream) terminalSize() (height uint, width uint) {
	if !s.tty {
		return 0, 0
	}
	ws, err := term.GetWinsize(s.fd)
	if err != nil {
		logrus.WithError(err).Debug("Error getting TTY size")
		if ws == nil {
			return 0, 0
		}
	}
	return uint(ws.Height), uint(ws.Width)
}

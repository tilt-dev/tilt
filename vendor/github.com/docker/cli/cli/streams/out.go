package streams

import (
	"io"
	"os"

	"github.com/moby/term"
	"github.com/sirupsen/logrus"
)

// Out is an output stream to write normal program output. It implements
// an [io.Writer], with additional utilities for detecting whether a terminal
// is connected, getting the TTY size, and putting the terminal in raw mode.
type Out struct {
	commonStream
	out io.Writer
}

func (o *Out) Write(p []byte) (int, error) {
	return o.out.Write(p)
}

// SetRawTerminal puts the output of the terminal connected to the stream
// into raw mode.
//
// On UNIX, this does nothing. On Windows, it disables LF -> CRLF/ translation.
// It is a no-op if Out is not a TTY, or if the "NORAW" environment variable is
// set to a non-empty value.
func (o *Out) SetRawTerminal() (err error) {
	if !o.isTerminal || os.Getenv("NORAW") != "" {
		return nil
	}
	o.state, err = term.SetRawTerminalOutput(o.fd)
	return err
}

// GetTtySize returns the height and width in characters of the TTY, or
// zero for both if no TTY is connected.
func (o *Out) GetTtySize() (height uint, width uint) {
	if !o.isTerminal {
		return 0, 0
	}
	ws, err := term.GetWinsize(o.fd)
	if err != nil {
		logrus.WithError(err).Debug("Error getting TTY size")
		if ws == nil {
			return 0, 0
		}
	}
	return uint(ws.Height), uint(ws.Width)
}

// NewOut returns a new [Out] from an [io.Writer].
func NewOut(out io.Writer) *Out {
	o := &Out{out: out}
	o.fd, o.isTerminal = term.GetFdInfo(out)
	return o
}

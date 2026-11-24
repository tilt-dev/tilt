package streams

import (
	"io"
	"os"

	"github.com/moby/term"
)

// Out is an output stream to write normal program output. It implements
// an [io.Writer], with additional utilities for detecting whether a terminal
// is connected, getting the TTY size, and putting the terminal in raw mode.
type Out struct {
	out io.Writer
	cs  commonStream
}

// NewOut returns a new [Out] from an [io.Writer]. If out is an [*os.File],
// a reference is kept to the file, and accessible through [Out.File].
func NewOut(out io.Writer) *Out {
	return &Out{
		out: out,
		cs:  newCommonStream(out),
	}
}

// FD returns the file descriptor number for this stream.
func (o *Out) FD() uintptr {
	return o.cs.FD()
}

// File returns the underlying *os.File if the stream was constructed from one.
// If the stream was created from a non-file (e.g., a pipe, buffer, or wrapper),
// the returned boolean will be false.
func (o *Out) File() (*os.File, bool) {
	return o.cs.file()
}

// Write writes to the output stream.
func (o *Out) Write(p []byte) (int, error) {
	return o.out.Write(p)
}

// IsTerminal returns whether this stream is connected to a terminal.
func (o *Out) IsTerminal() bool {
	return o.cs.isTerminal()
}

// SetRawTerminal puts the output of the terminal connected to the stream
// into raw mode.
//
// On UNIX, this does nothing. On Windows, it disables LF -> CRLF/ translation.
// It is a no-op if Out is not a TTY, or if the "NORAW" environment variable is
// set to a non-empty value.
func (o *Out) SetRawTerminal() error {
	return o.cs.setRawTerminal(term.SetRawTerminalOutput)
}

// RestoreTerminal restores the terminal state if SetRawTerminal succeeded earlier.
func (o *Out) RestoreTerminal() {
	o.cs.restoreTerminal()
}

// GetTtySize returns the height and width in characters of the TTY, or
// zero for both if no TTY is connected.
func (o *Out) GetTtySize() (height uint, width uint) {
	return o.cs.terminalSize()
}

// SetIsTerminal overrides whether a terminal is connected. It is used to
// override this property in unit-tests, and should not be depended on for
// other purposes.
func (o *Out) SetIsTerminal(isTerminal bool) {
	o.cs.setIsTerminal(isTerminal)
}

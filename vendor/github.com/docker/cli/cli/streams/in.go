package streams

import (
	"errors"
	"io"
	"os"

	"github.com/moby/term"
)

// In is an input stream to read user input. It implements [io.ReadCloser]
// with additional utilities, such as putting the terminal in raw mode.
type In struct {
	in io.ReadCloser
	cs commonStream
}

// NewIn returns a new [In] from an [io.ReadCloser]. If in is an [*os.File],
// a reference is kept to the file, and accessible through [In.File].
func NewIn(in io.ReadCloser) *In {
	return &In{
		in: in,
		cs: newCommonStream(in),
	}
}

// FD returns the file descriptor number for this stream.
func (i *In) FD() uintptr {
	return i.cs.fd
}

// File returns the underlying *os.File if the stream was constructed from one.
// If the stream was created from a non-file (e.g., a pipe, buffer, or wrapper),
// the returned boolean will be false.
func (i *In) File() (*os.File, bool) {
	return i.cs.file()
}

// Read implements the [io.Reader] interface.
func (i *In) Read(p []byte) (int, error) {
	return i.in.Read(p)
}

// Close implements the [io.Closer] interface.
func (i *In) Close() error {
	return i.in.Close()
}

// IsTerminal returns whether this stream is connected to a terminal.
func (i *In) IsTerminal() bool {
	return i.cs.isTerminal()
}

// SetRawTerminal sets raw mode on the input terminal. It is a no-op if In
// is not a TTY, or if the "NORAW" environment variable is set to a non-empty
// value.
func (i *In) SetRawTerminal() error {
	return i.cs.setRawTerminal(term.SetRawTerminal)
}

// RestoreTerminal restores the terminal state if SetRawTerminal succeeded earlier.
func (i *In) RestoreTerminal() {
	i.cs.restoreTerminal()
}

// CheckTty reports an error when stdin is requested for a TTY-enabled
// container, but the client stdin is not itself a terminal (for example,
// when input is piped or redirected).
func (i *In) CheckTty(attachStdin, ttyMode bool) error {
	// TODO(thaJeztah): consider inlining this code and deprecating the method.
	if !ttyMode || !attachStdin || i.cs.isTerminal() {
		return nil
	}
	return errors.New("cannot attach stdin to a TTY-enabled container because stdin is not a terminal")
}

// SetIsTerminal overrides whether a terminal is connected. It is used to
// override this property in unit-tests, and should not be depended on for
// other purposes.
func (i *In) SetIsTerminal(isTerminal bool) {
	i.cs.setIsTerminal(isTerminal)
}

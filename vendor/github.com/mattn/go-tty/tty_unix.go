//go:build !windows && !plan9
// +build !windows,!plan9

package tty

import (
	"bufio"
	"os"
	"os/signal"

	"golang.org/x/sys/unix"
)

type TTY struct {
	in      *os.File
	bin     *bufio.Reader
	out     *os.File
	termios unix.Termios
	ss      chan os.Signal
}

func open(path string) (*TTY, error) {
	tty := new(TTY)

	in, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	tty.in = in
	tty.bin = bufio.NewReader(in)

	out, err := os.OpenFile(path, unix.O_WRONLY, 0)
	if err != nil {
		return nil, err
	}
	tty.out = out

	termios, err := unix.IoctlGetTermios(int(tty.in.Fd()), ioctlReadTermios)
	if err != nil {
		return nil, err
	}
	tty.termios = *termios

	termios.Iflag &^= unix.ISTRIP | unix.INLCR | unix.ICRNL | unix.IGNCR | unix.IXOFF
	termios.Lflag &^= unix.ECHO | unix.ICANON /*| unix.ISIG*/
	termios.Cc[unix.VMIN] = 1
	termios.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(int(tty.in.Fd()), ioctlWriteTermios, termios); err != nil {
		return nil, err
	}

	tty.ss = make(chan os.Signal, 1)

	return tty, nil
}

func (tty *TTY) buffered() bool {
	return tty.bin.Buffered() > 0
}

func (tty *TTY) readRune() (rune, error) {
	r, _, err := tty.bin.ReadRune()
	return r, err
}

func (tty *TTY) close() error {
	signal.Stop(tty.ss)
	close(tty.ss)
	return unix.IoctlSetTermios(int(tty.in.Fd()), ioctlWriteTermios, &tty.termios)
}

func (tty *TTY) size() (int, int, error) {
	x, y, _, _, err := tty.sizePixel()
	return x, y, err
}

func (tty *TTY) sizePixel() (int, int, int, int, error) {
	ws, err := unix.IoctlGetWinsize(int(tty.out.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return -1, -1, -1, -1, err
	}
	return int(ws.Row), int(ws.Col), int(ws.Xpixel), int(ws.Ypixel), nil
}

func (tty *TTY) input() *os.File {
	return tty.in
}

func (tty *TTY) output() *os.File {
	return tty.out
}

func (tty *TTY) raw() (func() error, error) {
	termios, err := unix.IoctlGetTermios(int(tty.in.Fd()), ioctlReadTermios)
	if err != nil {
		return nil, err
	}
	backup := *termios

	termios.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	termios.Oflag &^= unix.OPOST
	termios.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	termios.Cflag &^= unix.CSIZE | unix.PARENB
	termios.Cflag |= unix.CS8
	termios.Cc[unix.VMIN] = 1
	termios.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(int(tty.in.Fd()), ioctlWriteTermios, termios); err != nil {
		return nil, err
	}

	return func() error {
		if err := unix.IoctlSetTermios(int(tty.in.Fd()), ioctlWriteTermios, &backup); err != nil {
			return err
		}
		return nil
	}, nil
}

func (tty *TTY) sigwinch() <-chan WINSIZE {
	signal.Notify(tty.ss, unix.SIGWINCH)

	ws := make(chan WINSIZE)
	go func() {
		defer close(ws)
		for sig := range tty.ss {
			if sig != unix.SIGWINCH {
				continue
			}

			w, h, err := tty.size()
			if err != nil {
				continue
			}
			// send but do not block for it
			select {
			case ws <- WINSIZE{W: w, H: h}:
			default:
			}

		}
	}()
	return ws
}

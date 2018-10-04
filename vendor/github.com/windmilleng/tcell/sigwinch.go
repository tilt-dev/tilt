package tcell

import (
	"os"
	"os/signal"
	"syscall"
)

type sigwinch struct {
	ch chan os.Signal

	// true if the channel is reporting SIGWINCH's from a tty other than the current one
	isRemote bool
}

func defaultSigwinch() sigwinch {
	return sigwinch{ch: make(chan os.Signal, 10)}
}

func sigwinchFromRemoteChan(ch chan os.Signal) sigwinch {
	if ch == nil {
		// No remote channel, return the default.
		return defaultSigwinch()
	}

	return sigwinch{
		ch:       ch,
		isRemote: true,
	}
}

// Notify sets the sigwinch channel to listen for SIGWINCH signals
// (unless it's a remote channel, i.e. already listening elsewhere,
// in which case, Notify does nothing).
func (s sigwinch) Notify() {
	signal.Notify(s.ch, syscall.SIGWINCH)
}

// Stop stops the sigwinch channel from relaying SIGWINCH signals
// (unless it's a remote channel, i.e. already listening elsewhere,
// in which case, Stop does nothing).
func (s sigwinch) Stop() {
	signal.Stop(s.ch)
}

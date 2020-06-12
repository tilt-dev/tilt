package store

// When Tilt is talking to a terminal, it can take on a few different modes.
type TerminalMode int

const (
	// A termbox UI takes over your terminal screen.
	TerminalModeHUD TerminalMode = iota

	// Logs are incrementally written to stdout.
	// This is the only available mode if the user
	// is redirecting tilt output to a file.
	TerminalModeStream

	// Tilt waits on a prompt to decide what mode
	// to be in.
	TerminalModePrompt
)

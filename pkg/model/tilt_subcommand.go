package model

// e.g., "up", "down", "ci"
type TiltSubcommand string

func (t TiltSubcommand) String() string {
	return string(t)
}

// Whether this subcommand binds the Tilt web port, holding it for as long as
// it runs. These are the subcommands that start an apiserver; everything else
// is a one-shot that binds nothing.
func (t TiltSubcommand) BindsWebPort() bool {
	switch t {
	case "up", "ci", "updog":
		return true
	}
	return false
}

package model

// e.g., "up", "down", "ci"
type TiltSubcommand string

func (t TiltSubcommand) String() string {
	return string(t)
}

package model

import "time"

type FlagsState struct {
	ConfigPath    string
	LastArgsWrite time.Time
	Args          []string
}

func NewFlagsState(args []string) FlagsState {
	return FlagsState{Args: args}
}

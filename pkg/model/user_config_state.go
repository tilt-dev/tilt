package model

import "time"

type UserConfigState struct {
	ArgsChangeTime time.Time
	Args           []string
}

func NewUserConfigState(args []string) UserConfigState {
	return UserConfigState{Args: args}
}

func (ucs UserConfigState) WithArgs(args []string) UserConfigState {
	ucs.Args = args
	ucs.ArgsChangeTime = time.Now()
	return ucs
}

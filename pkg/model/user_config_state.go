package model

type UserConfigState struct {
	Args []string
}

func NewUserConfigState(args []string) UserConfigState {
	return UserConfigState{Args: args}
}

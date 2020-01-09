package model

var DefaultMaxBuildSlots = 1

type UpdateSettings struct {
	MaxBuildSlots int // max number of builds to run concurrently
}

func DefaultUpdateSettings() UpdateSettings {
	return UpdateSettings{MaxBuildSlots: DefaultMaxBuildSlots}
}

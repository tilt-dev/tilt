package model

var DefaultMaxBuildSlots = 1

type UpdateSettings struct {
	MaxBuildSlots int // max number of builds to run concurrently
}

func (us UpdateSettings) MaxBuildSlotsMinOne() int {
	if us.MaxBuildSlots < 1 {
		return 1
	}
	return us.MaxBuildSlots
}

func DefaultUpdateSettings() UpdateSettings {
	return UpdateSettings{MaxBuildSlots: DefaultMaxBuildSlots}
}

package model

var DefaultMaxParallelUpdates = 1

type UpdateSettings struct {
	MaxParallelUpdates int // max number of builds to run concurrently
}

func (us UpdateSettings) MaxParallelUpdatesMinOne() int {
	if us.MaxParallelUpdates < 1 {
		return 1
	}
	return us.MaxParallelUpdates
}

func DefaultUpdateSettings() UpdateSettings {
	return UpdateSettings{MaxParallelUpdates: DefaultMaxParallelUpdates}
}

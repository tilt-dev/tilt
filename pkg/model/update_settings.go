package model

var DefaultMaxParallelUpdates = 3

type UpdateSettings struct {
	MaxParallelUpdates int // max number of updates to run concurrently
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

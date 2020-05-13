package model

var DefaultMaxParallelUpdates = 3

type UpdateSettings struct {
	maxParallelUpdates int // max number of updates to run concurrently
}

func (us UpdateSettings) MaxParallelUpdates() int {
	// Min. value is 1
	if us.maxParallelUpdates < 1 {
		return 1
	}
	return us.maxParallelUpdates
}

func (us UpdateSettings) WithMaxParallelUpdates(n int) UpdateSettings {
	// Min. value is 1
	if n < 1 {
		n = 1
	}
	us.maxParallelUpdates = n
	return us
}

func DefaultUpdateSettings() UpdateSettings {
	return UpdateSettings{maxParallelUpdates: DefaultMaxParallelUpdates}
}

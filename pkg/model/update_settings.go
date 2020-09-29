package model

import "time"

const (
	DefaultMaxParallelUpdates = 3
	DefaultK8sUpsertTimeout   = 30 * time.Second
)

type UpdateSettings struct {
	maxParallelUpdates int           // max number of updates to run concurrently
	k8sUpsertTimeout   time.Duration // timeout for k8s upsert operations
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

func (us UpdateSettings) K8sUpsertTimeout() time.Duration {
	// Min. value is 1s
	if us.k8sUpsertTimeout < time.Second {
		return time.Second
	}
	return us.k8sUpsertTimeout
}

func (us UpdateSettings) WithK8sUpsertTimeout(timeout time.Duration) UpdateSettings {
	// Min. value is 1s
	if us.k8sUpsertTimeout < time.Second {
		timeout = time.Second
	}
	us.k8sUpsertTimeout = timeout
	return us
}

func DefaultUpdateSettings() UpdateSettings {
	return UpdateSettings{
		maxParallelUpdates: DefaultMaxParallelUpdates,
		k8sUpsertTimeout:   DefaultK8sUpsertTimeout,
	}
}

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const BaseDir = "/base/directory"

func TestNewLiveUpdate(t *testing.T) {
	steps := []LiveUpdateStep{
		LiveUpdateSyncStep{"foo", "bar"},
		LiveUpdateRunStep{Cmd{[]string{"hello"}}, NewPathSet([]string{"goodbye"}, BaseDir)},
		LiveUpdateRestartContainerStep{},
	}
	fullRebuildTriggers := NewPathSet([]string{"quu", "qux"}, BaseDir)
	lu, err := NewLiveUpdate(steps, fullRebuildTriggers)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, LiveUpdate{steps, fullRebuildTriggers}, lu)
}

func TestNewLiveUpdateRestartContainerNotLast(t *testing.T) {
	steps := []LiveUpdateStep{LiveUpdateRestartContainerStep{}, LiveUpdateSyncStep{"foo", "bar"}}
	_, err := NewLiveUpdate(steps, PathSet{})
	if !assert.Error(t, err) {
		return
	}
	assert.Contains(t, err.Error(), "restart container is only valid as the last step")
}

func TestNewLiveUpdateSyncAfterRun(t *testing.T) {
	steps := append([]LiveUpdateStep{LiveUpdateRunStep{}, LiveUpdateSyncStep{"foo", "bar"}})
	_, err := NewLiveUpdate(steps, PathSet{})
	if !assert.Error(t, err) {
		return
	}
	assert.Contains(t, err.Error(), "all sync steps must precede all run steps")
}

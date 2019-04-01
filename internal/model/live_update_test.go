package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLiveUpdate(t *testing.T) {
	steps := []LiveUpdateStep{
		LiveUpdateSyncStep{"foo", "bar"},
		LiveUpdateRunStep{Cmd{[]string{"hello"}}, []string{"goodbye"}},
		LiveUpdateRestartContainerStep{},
	}
	fullRebuildTriggers := []string{"quu", "qux"}
	lu, err := NewLiveUpdate(
		steps,
		fullRebuildTriggers)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, LiveUpdate{steps, fullRebuildTriggers}, lu)
}

func TestNewLiveUpdateRestartContainerNotLast(t *testing.T) {
	steps := []LiveUpdateStep{LiveUpdateRestartContainerStep{}, LiveUpdateSyncStep{"foo", "bar"}}
	_, err := NewLiveUpdate(steps, []string{})
	if !assert.Error(t, err) {
		return
	}
	assert.Contains(t, err.Error(), "restart container is only valid as the last step")
}

func TestNewLiveUpdateSyncAfterRun(t *testing.T) {
	steps := append([]LiveUpdateStep{LiveUpdateRunStep{}, LiveUpdateSyncStep{"foo", "bar"}})
	_, err := NewLiveUpdate(steps, []string{})
	if !assert.Error(t, err) {
		return
	}
	assert.Contains(t, err.Error(), "all sync steps must precede all run steps")
}

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const BaseDir = "/base/directory"

func TestNewLiveUpdate(t *testing.T) {
	steps := []LiveUpdateStep{
		LiveUpdateFallBackOnStep{[]string{"quu", "qux"}},
		LiveUpdateSyncStep{"foo", "bar"},
		LiveUpdateRunStep{Cmd{[]string{"hello"}}, NewPathSet([]string{"goodbye"}, BaseDir)},
		LiveUpdateRestartContainerStep{},
	}
	lu, err := NewLiveUpdate(steps, BaseDir)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, LiveUpdate{steps, BaseDir}, lu)
}

func TestNewLiveUpdateRestartContainerNotLast(t *testing.T) {
	steps := []LiveUpdateStep{LiveUpdateRestartContainerStep{}, LiveUpdateSyncStep{"foo", "bar"}}
	_, err := NewLiveUpdate(steps, BaseDir)
	if !assert.Error(t, err) {
		return
	}
	assert.Contains(t, err.Error(), "restart container is only valid as the last step")
}

func TestNewLiveUpdateSyncAfterRun(t *testing.T) {
	steps := []LiveUpdateStep{LiveUpdateRunStep{}, LiveUpdateSyncStep{"foo", "bar"}}
	_, err := NewLiveUpdate(steps, BaseDir)
	if !assert.Error(t, err) {
		return
	}
	assert.Contains(t, err.Error(), "all sync steps must precede all run steps")
}

func TestNewLiveUpdateFallBackOnStepsNotFirst(t *testing.T) {
	steps := []LiveUpdateStep{
		LiveUpdateFallBackOnStep{[]string{"a"}},
		LiveUpdateSyncStep{"foo", "bar"},
		LiveUpdateFallBackOnStep{[]string{"b", "c"}},
		LiveUpdateSyncStep{"baz", "qux"},
	}
	_, err := NewLiveUpdate(steps, BaseDir)
	if !assert.Error(t, err) {
		return
	}
	assert.Contains(t, err.Error(), "all fall_back_on steps must precede all other steps")
}

func TestNewLiveUpdateFallBackOnFiles(t *testing.T) {
	steps := []LiveUpdateStep{
		LiveUpdateFallBackOnStep{[]string{"a"}},
		LiveUpdateFallBackOnStep{[]string{"b", "c"}},
		LiveUpdateFallBackOnStep{[]string{"d"}},
	}
	lu, err := NewLiveUpdate(steps, BaseDir)
	if !assert.NoError(t, err) {
		return
	}
	expectedFallBackFiles := NewPathSet([]string{"a", "b", "c", "d"}, BaseDir)
	assert.Equal(t, expectedFallBackFiles, lu.FallBackOnFiles())
}

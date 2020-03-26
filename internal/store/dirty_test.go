// +build !windows

package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDirtyBitBasic(t *testing.T) {
	d := NewDirtyBit()
	assert.False(t, d.IsDirty())

	_, dirty := d.StartBuildIfDirty()
	assert.False(t, dirty)

	d.MarkDirty()

	token, dirty := d.StartBuildIfDirty()
	assert.True(t, dirty)
	assert.True(t, d.IsDirty())

	d.FinishBuild(token)
	assert.False(t, d.IsDirty())
}

func TestDirtyBitDuringBuild(t *testing.T) {
	d := NewDirtyBit()
	assert.False(t, d.IsDirty())

	_, dirty := d.StartBuildIfDirty()
	assert.False(t, dirty)

	d.MarkDirty()

	token, dirty := d.StartBuildIfDirty()
	assert.True(t, dirty)
	assert.True(t, d.IsDirty())

	d.MarkDirty()

	d.FinishBuild(token)
	assert.True(t, d.IsDirty())
}

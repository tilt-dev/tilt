package model

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func TestOneImageGraph(t *testing.T) {
	iTarget := newDepTarget("image-a")
	kTarget := newK8sTarget("fe", "image-a")
	g, err := NewTargetGraph([]TargetSpec{
		iTarget,
		kTarget,
	})
	assert.NoError(t, err)
	assert.True(t, g.IsSingleSourceDAG())
	assert.Equal(t, 1, len(g.DeployedImages()))

	ids := []TargetID{}
	err = g.VisitTree(kTarget, func(t TargetSpec) error {
		ids = append(ids, t.ID())
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []TargetID{iTarget.ID(), kTarget.ID()}, ids)
}

func TestTwoImageGraph(t *testing.T) {
	targetA := newDepTarget("image-a")
	targetB := newDepTarget("image-b")
	kTarget := newK8sTarget("fe", "image-a", "image-b")
	g, err := NewTargetGraph([]TargetSpec{
		targetA,
		targetB,
		kTarget,
	})
	assert.NoError(t, err)
	assert.True(t, g.IsSingleSourceDAG())
	assert.Equal(t, 2, len(g.DeployedImages()))

	ids := []TargetID{}
	err = g.VisitTree(kTarget, func(t TargetSpec) error {
		ids = append(ids, t.ID())
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []TargetID{targetA.ID(), targetB.ID(), kTarget.ID()}, ids)
}

func TestDependentImageGraph(t *testing.T) {
	targetA := newDepTarget("image-a")
	targetB := newDepTarget("image-b", "image-a")
	kTarget := newK8sTarget("fe", "image-b")
	g, err := NewTargetGraph([]TargetSpec{
		targetA,
		targetB,
		kTarget,
	})
	assert.NoError(t, err)
	assert.True(t, g.IsSingleSourceDAG())
	assert.Equal(t, 1, len(g.DeployedImages()))

	ids := []TargetID{}
	err = g.VisitTree(kTarget, func(t TargetSpec) error {
		ids = append(ids, t.ID())
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []TargetID{targetA.ID(), targetB.ID(), kTarget.ID()}, ids)
}

func TestDiamondImageGraph(t *testing.T) {
	targetA := newDepTarget("image-a")
	targetB := newDepTarget("image-b", "image-a")
	targetC := newDepTarget("image-c", "image-a")
	targetD := newDepTarget("image-d", "image-b", "image-c")
	kTarget := newK8sTarget("fe", "image-d")
	g, err := NewTargetGraph([]TargetSpec{
		targetA,
		targetB,
		targetC,
		targetD,
		kTarget,
	})
	assert.NoError(t, err)
	assert.True(t, g.IsSingleSourceDAG())
	assert.Equal(t, 1, len(g.DeployedImages()))

	ids := []TargetID{}
	err = g.VisitTree(kTarget, func(t TargetSpec) error {
		ids = append(ids, t.ID())
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []TargetID{targetA.ID(), targetB.ID(), targetC.ID(), targetD.ID(), kTarget.ID()}, ids)
}

func TestTwoDeployGraph(t *testing.T) {
	kTargetA := newK8sTarget("fe-a")
	kTargetB := newK8sTarget("fe-b")
	g, err := NewTargetGraph([]TargetSpec{
		kTargetA,
		kTargetB,
	})
	assert.NoError(t, err)
	assert.False(t, g.IsSingleSourceDAG())
	assert.Equal(t, 0, len(g.DeployedImages()))
}

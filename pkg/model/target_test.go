package model

import (
	"fmt"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/container"
)

type topSortCase struct {
	inputs  []TargetSpec
	outputs []string
	err     string
}

func TestTopSort(t *testing.T) {
	cases := []topSortCase{
		topSortCase{
			inputs: []TargetSpec{newDepTarget("a", "b")},
			err:    "Missing target dependency: b",
		},
		topSortCase{
			inputs: []TargetSpec{
				newDepTarget("a", "b"),
				newDepTarget("b", "a"),
			},
			err: "Found a cycle at target: a",
		},
		topSortCase{
			inputs: []TargetSpec{
				newDepTarget("a", "b"),
				newDepTarget("b", "c"),
				newDepTarget("c", "a"),
			},
			err: "Found a cycle at target: a",
		},
		topSortCase{
			inputs: []TargetSpec{
				newDepTarget("a", "b"),
				newDepTarget("b", "c"),
				newDepTarget("c"),
			},
			outputs: []string{"c", "b", "a"},
		},
		topSortCase{
			inputs: []TargetSpec{
				newDepTarget("a", "b", "d"),
				newDepTarget("b", "d"),
				newDepTarget("c", "d"),
				newDepTarget("d"),
			},
			outputs: []string{"d", "b", "a", "c"},
		},
		topSortCase{
			inputs: []TargetSpec{
				newDepTarget("a", "b", "d"),
				newDepTarget("c", "d"),
				newDepTarget("b", "d"),
				newDepTarget("d"),
			},
			outputs: []string{"d", "b", "a", "c"},
		},
		topSortCase{
			inputs: []TargetSpec{
				newDepTarget("c", "d"),
				newDepTarget("b", "d"),
				newDepTarget("a", "b", "d"),
				newDepTarget("d"),
			},
			outputs: []string{"d", "c", "b", "a"},
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("TopSort%d", i), func(t *testing.T) {
			sorted, err := TopologicalSort(c.inputs)
			if err != nil {
				if c.err == "" {
					t.Fatalf("Unexpected error: %v", err)
					return
				}

				assert.Contains(t, err.Error(), c.err)
				return
			}

			if c.err != "" {
				t.Fatalf("Expected error: %s. Actual nil", c.err)
			}

			sortedIDs := make([]string, len(sorted))
			for i, t := range sorted {
				sortedIDs[i] = path.Base(t.ID().Name.String())
			}
			assert.Equal(t, c.outputs, sortedIDs)
		})
	}
}

func newDepTarget(name string, deps ...string) ImageTarget {
	ref := container.MustParseSelector(name)
	depIDs := make([]TargetID, len(deps))
	for i, dep := range deps {
		depIDs[i] = ImageID(container.MustParseSelector(dep))
	}
	return MustNewImageTarget(ref).WithDependencyIDs(depIDs)
}

func newK8sTarget(name string, deps ...string) K8sTarget {
	depIDs := make([]TargetID, len(deps))
	for i, dep := range deps {
		depIDs[i] = ImageID(container.MustParseSelector(dep))
	}
	return K8sTarget{Name: TargetName(name)}.WithDependencyIDs(depIDs)
}

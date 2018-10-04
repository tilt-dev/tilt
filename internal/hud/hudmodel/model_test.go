package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/hud/hudview"
	"github.com/windmilleng/tilt/internal/model"
)

const manifestName = model.ManifestName("vigoda")

func TestNewViewNormal(t *testing.T) {
	vtf := newViewTestFixture(t)
	vtf.run()
	vtf.assertStatus(hudview.ResourceStatusFresh, "Running")
}

func TestNewViewNoPod(t *testing.T) {
	vtf := newViewTestFixture(t)
	delete(vtf.model.Pods, manifestName)
	vtf.run()
	vtf.assertStatus(hudview.ResourceStatusStale, "No pod found")
}

func TestNewViewBrokenPod(t *testing.T) {
	vtf := newViewTestFixture(t)
	vtf.model.Pods[manifestName].Status = "CrashLoopBackOff"
	vtf.run()
	vtf.assertStatus(hudview.ResourceStatusBroken, "CrashLoopBackOff")
}

type viewTestFixture struct {
	t        *testing.T
	resource *Resource
	model    *upperState

	generatedView hudview.View
}

func newViewTestFixture(t *testing.T) *viewTestFixture {
	ts := time.Now()

	r := Resource{
		DirectoryWatched:   ".",
		LatestFileChanges:  []string{},
		LastFileChangeTime: ts,
		Status:             resourceStatusStale,
	}

	m := upperState{
		Resources: map[model.ManifestName]*Resource{
			manifestName: &r,
		},
		Pods: map[model.ManifestName]*Pod{
			manifestName: {
				Name:      "vigoda-12345abcd",
				StartedAt: ts,
				Status:    "Running",
			},
		},
	}
	return &viewTestFixture{t, &r, &m, hudview.View{}}
}

func (vtf *viewTestFixture) run() {
	ret := newView(*vtf.model)

	if !assert.Equal(vtf.t, 1, len(vtf.model.Resources)) {
		vtf.t.Fail()
	}

	vtf.generatedView = ret

	assert.Equal(vtf.t, vtf.resource.LatestFileChanges, ret.Resources[0].LatestFileChanges)
	assert.Equal(vtf.t, vtf.resource.DirectoryWatched, ret.Resources[0].DirectoryWatched)
}

func (vtf *viewTestFixture) assertStatus(expectedStatus hudview.ResourceStatus, desc string) {
	assert.Equal(vtf.t, expectedStatus, vtf.generatedView.Resources[0].Status)
	assert.Equal(vtf.t, desc, vtf.generatedView.Resources[0].StatusDesc)
}

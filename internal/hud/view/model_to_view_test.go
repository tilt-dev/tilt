package view

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	hudmodel "github.com/windmilleng/tilt/internal/hud/model"
	tiltmodel "github.com/windmilleng/tilt/internal/model"
)

const manifestName = tiltmodel.ManifestName("vigoda")

func TestNewViewNormal(t *testing.T) {
	vtf := newViewTestFixture(t)
	vtf.run()
	vtf.assertStatus(ResourceStatusFresh, "Running")
}

func TestNewViewNoPod(t *testing.T) {
	vtf := newViewTestFixture(t)
	delete(vtf.model.Pods, manifestName)
	vtf.run()
	vtf.assertStatus(ResourceStatusStale, "No pod found")
}

func TestNewViewBrokenPod(t *testing.T) {
	vtf := newViewTestFixture(t)
	vtf.model.Pods[manifestName].Status = "CrashLoopBackOff"
	vtf.run()
	vtf.assertStatus(ResourceStatusBroken, "CrashLoopBackOff")
}

type viewTestFixture struct {
	t        *testing.T
	resource *hudmodel.Resource
	model    *hudmodel.Model

	generatedView View
}

func newViewTestFixture(t *testing.T) *viewTestFixture {
	ts := time.Now()

	r := hudmodel.Resource{
		DirectoryWatched:   ".",
		LatestFileChanges:  []string{},
		LastFileChangeTime: ts,
		Status:             hudmodel.ResourceStatusStale,
	}

	m := hudmodel.Model{
		Resources: map[tiltmodel.ManifestName]*hudmodel.Resource{
			manifestName: &r,
		},
		Pods: map[tiltmodel.ManifestName]*hudmodel.Pod{
			manifestName: {
				Name:      "vigoda-12345abcd",
				StartedAt: ts,
				Status:    "Running",
			},
		},
	}
	return &viewTestFixture{t, &r, &m, View{}}
}

func (vtf *viewTestFixture) run() {
	ret := NewView(*vtf.model)

	if !assert.Equal(vtf.t, 1, len(vtf.model.Resources)) {
		vtf.t.Fail()
	}

	vtf.generatedView = ret

	assert.Equal(vtf.t, vtf.resource.LatestFileChanges, ret.Resources[0].LatestFileChanges)
	assert.Equal(vtf.t, vtf.resource.DirectoryWatched, ret.Resources[0].DirectoryWatched)
}

func (vtf *viewTestFixture) assertStatus(expectedStatus ResourceStatus, desc string) {
	assert.Equal(vtf.t, expectedStatus, vtf.generatedView.Resources[0].Status)
	assert.Equal(vtf.t, desc, vtf.generatedView.Resources[0].StatusDesc)
}

package store

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/hud/view"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/model"
)

type endpointsCase struct {
	name     string
	expected []model.Link

	// k8s resource fields
	portFwds []model.PortForward
	lbURLs   []string

	dcPublishedPorts []int

	k8sResLinks   []model.Link
	localResLinks []model.Link
}

func (c endpointsCase) validate() {
	if len(c.portFwds) > 0 || len(c.lbURLs) > 0 || len(c.k8sResLinks) > 0 {
		if len(c.dcPublishedPorts) > 0 || len(c.localResLinks) > 0 {
			// portForwards and LoadBalancerURLs are exclusively the province
			// of k8s resources, so you should never see them paired with
			// test settings that imply a. a DC resource or b. a local resource
			panic("test case implies impossible resource")
		}
	}
}

func TestStateToViewRelativeEditPaths(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()
	m := model.Manifest{
		Name: "foo",
	}.WithDeployTarget(model.K8sTarget{}).WithImageTarget(model.ImageTarget{}.
		WithBuildDetails(model.DockerBuild{BuildPath: f.JoinPath("a", "b", "c")}))

	state := newState([]model.Manifest{m})
	ms := state.ManifestTargets[m.Name].State
	ms.CurrentBuild.Edits = []string{
		f.JoinPath("a", "b", "c", "foo"),
		f.JoinPath("a", "b", "c", "d", "e")}
	ms.BuildHistory = []model.BuildRecord{
		{
			Edits: []string{
				f.JoinPath("a", "b", "c", "foo"),
				f.JoinPath("a", "b", "c", "d", "e"),
			},
		},
	}
	ms.MutableBuildStatus(m.ImageTargets[0].ID()).PendingFileChanges =
		map[string]time.Time{
			f.JoinPath("a", "b", "c", "foo"):    time.Now(),
			f.JoinPath("a", "b", "c", "d", "e"): time.Now(),
		}
	v := StateToView(*state, &sync.RWMutex{})

	require.Len(t, v.Resources, 2)

	r, _ := v.Resource(m.Name)
	assert.Equal(t, []string{"foo", filepath.Join("d", "e")}, r.LastBuild().Edits)
	assert.Equal(t, []string{"foo", filepath.Join("d", "e")}, r.CurrentBuild.Edits)
	assert.Equal(t, []string{filepath.Join("d", "e"), "foo"}, r.PendingBuildEdits) // these are sorted for deterministic ordering
}

func TestStateToViewPortForwards(t *testing.T) {
	m := model.Manifest{
		Name: "foo",
	}.WithDeployTarget(model.K8sTarget{
		PortForwards: []model.PortForward{
			{LocalPort: 8000, ContainerPort: 5000},
			{LocalPort: 7000, ContainerPort: 5001},
		},
	})
	state := newState([]model.Manifest{m})
	v := StateToView(*state, &sync.RWMutex{})
	res, _ := v.Resource(m.Name)
	assert.Equal(t,
		[]string{"http://localhost:7000/", "http://localhost:8000/"},
		res.Endpoints)
}

func TestStateToViewLocalResourceLinks(t *testing.T) {
	m := model.Manifest{
		Name: "foo",
	}.WithDeployTarget(model.LocalTarget{
		Links: []model.Link{
			model.MustNewLink("www.zombo.com", "zombo"),
			model.MustNewLink("www.apple.edu", "apple"),
		},
	})
	state := newState([]model.Manifest{m})
	v := StateToView(*state, &sync.RWMutex{})
	res, _ := v.Resource(m.Name)
	assert.Equal(t,
		[]string{"www.apple.edu", "www.zombo.com"},
		res.Endpoints)
}

func TestRuntimeStateNonWorkload(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()

	m := manifestbuilder.New(f, model.UnresourcedYAMLManifestName).
		WithK8sYAML(testyaml.SecretYaml).
		Build()
	state := newState([]model.Manifest{m})
	runtimeState := state.ManifestTargets[m.Name].State.K8sRuntimeState()
	assert.Equal(t, model.RuntimeStatusPending, runtimeState.RuntimeStatus())

	runtimeState.HasEverDeployedSuccessfully = true

	assert.Equal(t, model.RuntimeStatusOK, runtimeState.RuntimeStatus())
}

func TestStateToViewUnresourcedYAMLManifest(t *testing.T) {
	m, err := k8s.NewK8sOnlyManifestFromYAML(testyaml.SanchoYAML)
	assert.NoError(t, err)
	state := newState([]model.Manifest{m})
	v := StateToView(*state, &sync.RWMutex{})

	assert.Equal(t, 2, len(v.Resources))

	r, _ := v.Resource(m.Name)
	assert.Equal(t, nil, r.LastBuild().Error)

	expectedInfo := view.YAMLResourceInfo{
		K8sDisplayNames: []string{"sancho:deployment"},
	}
	assert.Equal(t, expectedInfo, r.ResourceInfo)
}

func TestStateToViewNonWorkloadYAMLManifest(t *testing.T) {
	es, err := k8s.ParseYAMLFromString(testyaml.SecretYaml)
	require.NoError(t, err)
	m, err := k8s.NewK8sOnlyManifest(model.ManifestName("foo"), es, nil)
	require.NoError(t, err)
	state := newState([]model.Manifest{m})
	v := StateToView(*state, &sync.RWMutex{})

	assert.Equal(t, 2, len(v.Resources))

	r, _ := v.Resource(m.Name)
	assert.Equal(t, nil, r.LastBuild().Error)

	expectedInfo := view.YAMLResourceInfo{
		K8sDisplayNames: []string{"mysecret:secret"},
	}
	assert.Equal(t, expectedInfo, r.ResourceInfo)
}

func TestMostRecentPod(t *testing.T) {
	podA := Pod{PodID: "pod-a", StartedAt: time.Now()}
	podB := Pod{PodID: "pod-b", StartedAt: time.Now().Add(time.Minute)}
	podC := Pod{PodID: "pod-c", StartedAt: time.Now().Add(-time.Minute)}
	m := model.Manifest{Name: "fe"}
	podSet := NewK8sRuntimeStateWithPods(m, podA, podB, podC)
	assert.Equal(t, "pod-b", podSet.MostRecentPod().PodID.String())
}

func TestRelativeTiltfilePath(t *testing.T) {
	es := newState([]model.Manifest{})
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	es.TiltfilePath = filepath.Join(wd, "Tiltfile")

	actual, err := es.RelativeTiltfilePath()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "Tiltfile", actual)
}

func TestNextBuildReason(t *testing.T) {
	m, err := k8s.NewK8sOnlyManifestFromYAML(testyaml.SanchoYAML)
	assert.NoError(t, err)

	kTarget := m.K8sTarget()
	mt := NewManifestTarget(m)

	iTargetID := model.ImageID(container.MustParseSelector("sancho"))
	status := mt.State.MutableBuildStatus(kTarget.ID())
	assert.Equal(t, "Initial Build",
		mt.NextBuildReason().String())

	status.PendingDependencyChanges[iTargetID] = time.Now()
	assert.Equal(t, "Initial Build",
		mt.NextBuildReason().String())

	mt.State.AddCompletedBuild(model.BuildRecord{StartTime: time.Now(), FinishTime: time.Now()})
	assert.Equal(t, "Dependency Updated",
		mt.NextBuildReason().String())

	status.PendingFileChanges["a.txt"] = time.Now()
	assert.Equal(t, "Changed Files | Dependency Updated",
		mt.NextBuildReason().String())
}

func TestManifestTargetEndpoints(t *testing.T) {
	cases := []endpointsCase{
		{
			name: "port forward",
			expected: []model.Link{
				model.MustNewLink("http://localhost:7000/", ""),
				model.MustNewLink("http://localhost:8000/", "foobar"),
			},
			portFwds: []model.PortForward{
				{LocalPort: 8000, ContainerPort: 5000, Name: "foobar"},
				{LocalPort: 7000, ContainerPort: 5001},
			},
		},
		{
			name: "port forward with host",
			expected: []model.Link{
				model.MustNewLink("http://host1:8000/", "foobar"),
				model.MustNewLink("http://host2:7000/", ""),
			},
			portFwds: []model.PortForward{
				{LocalPort: 8000, ContainerPort: 5000, Host: "host1", Name: "foobar"},
				{LocalPort: 7000, ContainerPort: 5001, Host: "host2"},
			},
		},
		{
			name: "port forward with path",
			expected: []model.Link{
				model.MustNewLink("http://localhost:8000/stuff", "foobar"),
			},
			portFwds: []model.PortForward{
				model.MustPortForward(8000, 5000, "", "foobar", "stuff"),
			},
		},
		{
			name: "port forward with path trims leading slash",
			expected: []model.Link{
				model.MustNewLink("http://localhost:8000/v1/ui", "UI"),
			},
			portFwds: []model.PortForward{
				model.MustPortForward(8000, 0, "", "UI", "/v1/ui"),
			},
		},
		{
			name: "port forward with path and host",
			expected: []model.Link{
				model.MustNewLink("http://host1:8000/apple", "foobar"),
				model.MustNewLink("http://host2:7000/banana", ""),
			},
			portFwds: []model.PortForward{
				model.MustPortForward(8000, 5000, "host1", "foobar", "apple"),
				model.MustPortForward(7000, 5001, "host2", "", "/banana"),
			},
		},
		{
			name: "port forward and links",
			expected: []model.Link{
				// NOTE(maia): this is current sorting behavior (i.e. whole list of
				// endpoints is sorted) and isn't great; probably arbitrary links and
				// port forward URLs should be grouped together.
				model.MustNewLink("http://apple.edu", "apple"),
				model.MustNewLink("http://localhost:8000/", "foobar"),
				model.MustNewLink("www.zombo.com", "zombo"),
			},
			portFwds: []model.PortForward{
				{LocalPort: 8000, Name: "foobar"},
			},
			k8sResLinks: []model.Link{
				model.MustNewLink("http://apple.edu", "apple"),
				model.MustNewLink("www.zombo.com", "zombo"),
			},
		},
		{
			name: "local resource links",
			expected: []model.Link{
				model.MustNewLink("www.apple.edu", "apple"),
				model.MustNewLink("www.zombo.com", "zombo"),
			},
			localResLinks: []model.Link{
				model.MustNewLink("www.apple.edu", "apple"),
				model.MustNewLink("www.zombo.com", "zombo"),
			},
		},
		{
			name: "docker compose ports",
			expected: []model.Link{
				model.MustNewLink("http://localhost:7000/", ""),
				model.MustNewLink("http://localhost:8000/", ""),
			},
			dcPublishedPorts: []int{8000, 7000},
		},
		{
			name: "load balancers",
			expected: []model.Link{
				model.MustNewLink("www.apple.edu", ""),
				model.MustNewLink("www.zombo.com", ""),
			},
			lbURLs: []string{"www.zombo.com", "www.apple.edu"},
		},
		{
			name: "load balancers and links",
			expected: []model.Link{
				model.MustNewLink("www.apple.edu", "apple"),
				model.MustNewLink("www.zombo.com", ""),
			},
			lbURLs: []string{"www.zombo.com"},
			k8sResLinks: []model.Link{
				model.MustNewLink("www.apple.edu", "apple"),
			},
		},
		{
			name: "port forwards supercede LBs",
			expected: []model.Link{
				model.MustNewLink("http://localhost:7000/", ""),
			},
			portFwds: []model.PortForward{
				{LocalPort: 7000, ContainerPort: 5001},
			},
			lbURLs: []string{"www.zombo.com"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.validate()
			m := model.Manifest{Name: "foo"}

			if len(c.portFwds) > 0 || len(c.k8sResLinks) > 0 {
				m = m.WithDeployTarget(model.K8sTarget{
					PortForwards: c.portFwds,
					Links:        c.k8sResLinks,
				})
			} else if len(c.localResLinks) > 0 {
				m = m.WithDeployTarget(model.LocalTarget{Links: c.localResLinks})
			} else if len(c.dcPublishedPorts) > 0 {
				m = m.WithDeployTarget(model.DockerComposeTarget{}.WithPublishedPorts(c.dcPublishedPorts))
			}

			mt := newManifestTargetWithLoadBalancerURLs(m, c.lbURLs)
			actual := ManifestTargetEndpoints(mt)
			assert.Equal(t, c.expected, actual)
		})
	}
}

func newManifestTargetWithLoadBalancerURLs(m model.Manifest, urls []string) *ManifestTarget {
	mt := NewManifestTarget(m)
	if len(urls) == 0 {
		return mt
	}

	lbs := make(map[k8s.ServiceName]*url.URL)
	for i, s := range urls {
		u, err := url.Parse(s)
		if err != nil {
			panic(fmt.Sprintf("error parsing url %q for dummy load balancers: %v",
				s, err))
		}
		name := k8s.ServiceName(fmt.Sprintf("svc#%d", i))
		lbs[name] = u
	}
	k8sState := NewK8sRuntimeState(m)
	k8sState.LBs = lbs
	mt.State.RuntimeState = k8sState

	if !mt.Manifest.IsK8s() {
		// k8s state implies a k8s deploy target; if this manifest doesn't have one,
		// add a dummy one
		mt.Manifest = mt.Manifest.WithDeployTarget(model.K8sTarget{})
	}

	return mt
}

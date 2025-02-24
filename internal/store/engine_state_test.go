package store

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

type endpointsCase struct {
	name     string
	expected []model.Link

	// k8s resource fields
	portFwds []model.PortForward
	lbURLs   []string

	dcPublishedPorts []int
	dcPortBindings   []v1alpha1.DockerPortBinding

	k8sResLinks       []model.Link
	localResLinks     []model.Link
	dcResLinks        []model.Link
	dcDoNotInferLinks bool
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

func TestMostRecentPod(t *testing.T) {
	podA := v1alpha1.Pod{Name: "pod-a", CreatedAt: apis.Now()}
	podB := v1alpha1.Pod{Name: "pod-b", CreatedAt: apis.NewTime(time.Now().Add(time.Minute))}
	podC := v1alpha1.Pod{Name: "pod-c", CreatedAt: apis.NewTime(time.Now().Add(-time.Minute))}
	m := model.Manifest{Name: "fe"}
	podSet := NewK8sRuntimeStateWithPods(m, podA, podB, podC)
	assert.Equal(t, "pod-b", podSet.MostRecentPod().Name)
}

func TestNextBuildReason(t *testing.T) {
	m := k8sManifest(t, model.UnresourcedYAMLManifestName, testyaml.SanchoYAML)

	kTarget := m.K8sTarget()
	mt := NewManifestTarget(m)

	iTargetID := model.ImageID(container.MustParseSelector("sancho"))
	status := mt.State.MutableBuildStatus(kTarget.ID())
	assert.Equal(t, "Initial Build",
		mt.NextBuildReason().String())

	status.DependencyChanges[iTargetID] = time.Now()
	assert.Equal(t, "Initial Build",
		mt.NextBuildReason().String())

	mt.State.AddCompletedBuild(model.BuildRecord{StartTime: time.Now(), FinishTime: time.Now()})
	assert.Equal(t, "Dependency Updated",
		mt.NextBuildReason().String())

	status.FileChanges["a.txt"] = time.Now()
	assert.Equal(t, "Changed Files | Dependency Updated",
		mt.NextBuildReason().String())
}

func TestBuildStatusGC(t *testing.T) {
	start := time.Now()
	bs := newBuildStatus()
	assert.False(t, bs.HasPendingFileChanges())
	assert.False(t, bs.HasPendingDependencyChanges())

	bs.FileChanges["a.txt"] = start
	bs.FileChanges["b.txt"] = start
	bs.DependencyChanges[model.ImageID(container.MustParseSelector("sancho"))] = start

	assert.True(t, bs.HasPendingFileChanges())
	assert.True(t, bs.HasPendingDependencyChanges())
	assert.Equal(t, []string{"a.txt", "b.txt"}, bs.PendingFileChangesSorted())

	bs.ConsumeChangesBefore(start.Add(time.Second))
	assert.False(t, bs.HasPendingFileChanges())
	assert.False(t, bs.HasPendingDependencyChanges())
	assert.Equal(t, 2, len(bs.FileChanges))
	assert.Equal(t, 1, len(bs.DependencyChanges))
	assert.Equal(t, []string(nil), bs.PendingFileChangesSorted())

	bs.FileChanges["a.txt"] = start.Add(2 * time.Second)
	assert.True(t, bs.HasPendingFileChanges())

	bs.ConsumeChangesBefore(start.Add(time.Hour))
	assert.False(t, bs.HasPendingFileChanges())
	assert.False(t, bs.HasPendingDependencyChanges())

	// GC should remove sufficiently old changes from the map
	// since they'll just slow things down.
	assert.Equal(t, 0, len(bs.FileChanges))
	assert.Equal(t, 0, len(bs.DependencyChanges))
}

func TestManifestTargetEndpoints(t *testing.T) {
	cases := []endpointsCase{
		{
			name: "port forward",
			expected: []model.Link{
				model.MustNewLink("http://localhost:8000/", "foobar"),
				model.MustNewLink("http://localhost:7000/", ""),
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
				model.MustNewLink("www.zombo.com", "zombo"),
				model.MustNewLink("http://apple.edu", "apple"),
				model.MustNewLink("http://localhost:8000/", "foobar"),
			},
			portFwds: []model.PortForward{
				{LocalPort: 8000, Name: "foobar"},
			},
			k8sResLinks: []model.Link{
				model.MustNewLink("www.zombo.com", "zombo"),
				model.MustNewLink("http://apple.edu", "apple"),
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
				model.MustNewLink("http://localhost:8000/", ""),
				model.MustNewLink("http://localhost:7000/", ""),
			},
			dcPublishedPorts: []int{8000, 7000},
		},
		{
			name: "docker compose ports and links",
			expected: []model.Link{
				model.MustNewLink("http://localhost:8000/", ""),
				model.MustNewLink("http://localhost:7000/", ""),
				model.MustNewLink("www.apple.edu", "apple"),
				model.MustNewLink("www.zombo.com", "zombo"),
			},
			dcPublishedPorts: []int{8000, 7000},
			dcResLinks: []model.Link{
				model.MustNewLink("www.apple.edu", "apple"),
				model.MustNewLink("www.zombo.com", "zombo"),
			},
		},
		{
			name:              "docker compose ports with inferLinks=false",
			dcPublishedPorts:  []int{8000, 7000},
			dcDoNotInferLinks: true,
		},
		{
			name: "docker compose ports and links with inferLinks=false",
			expected: []model.Link{
				model.MustNewLink("www.apple.edu", "apple"),
				model.MustNewLink("www.zombo.com", "zombo"),
			},
			dcPublishedPorts: []int{8000, 7000},
			dcResLinks: []model.Link{
				model.MustNewLink("www.apple.edu", "apple"),
				model.MustNewLink("www.zombo.com", "zombo"),
			},
			dcDoNotInferLinks: true,
		},
		{
			name: "docker compose dynamic ports",
			expected: []model.Link{
				model.MustNewLink("http://localhost:8000/", ""),
			},
			dcPortBindings: []v1alpha1.DockerPortBinding{
				{
					ContainerPort: 8080,
					HostIP:        "0.0.0.0",
					HostPort:      8000,
				},
				{
					ContainerPort: 8080,
					HostIP:        "::",
					HostPort:      8000,
				},
			},
		},
		{
			name: "load balancers",
			expected: []model.Link{
				model.MustNewLink("a", ""), model.MustNewLink("b", ""), model.MustNewLink("c", ""), model.MustNewLink("d", ""),
				model.MustNewLink("w", ""), model.MustNewLink("x", ""), model.MustNewLink("y", ""), model.MustNewLink("z", ""),
			},
			// this is where we have some room for non-determinism, so maximize the chance of something going wrong
			lbURLs: []string{"z", "y", "x", "w", "d", "c", "b", "a"},
		},
		{
			name: "load balancers and links",
			expected: []model.Link{
				model.MustNewLink("www.zombo.com", "zombo"),
				model.MustNewLink("www.apple.edu", ""),
				model.MustNewLink("www.banana.com", ""),
			},
			lbURLs: []string{"www.banana.com", "www.apple.edu"},
			k8sResLinks: []model.Link{
				model.MustNewLink("www.zombo.com", "zombo"),
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
				var forwards []v1alpha1.Forward
				for _, pf := range c.portFwds {
					forwards = append(forwards, v1alpha1.Forward{
						LocalPort:     int32(pf.LocalPort),
						ContainerPort: int32(pf.ContainerPort),
						Host:          pf.Host,
						Name:          pf.Name,
						Path:          pf.PathForAppend(),
					})
				}

				m = m.WithDeployTarget(model.K8sTarget{
					KubernetesApplySpec: v1alpha1.KubernetesApplySpec{
						PortForwardTemplateSpec: &v1alpha1.PortForwardTemplateSpec{
							Forwards: forwards,
						},
					},
					Links: c.k8sResLinks,
				})
			} else if len(c.localResLinks) > 0 {
				m = m.WithDeployTarget(model.LocalTarget{Links: c.localResLinks})
			}

			isDC := len(c.dcPublishedPorts) > 0 || len(c.dcResLinks) > 0

			if isDC {
				dockerDeployTarget := model.DockerComposeTarget{}

				if len(c.dcPublishedPorts) > 0 {
					dockerDeployTarget = dockerDeployTarget.WithPublishedPorts(c.dcPublishedPorts)
				}

				if len(c.dcResLinks) > 0 {
					dockerDeployTarget.Links = c.dcResLinks
				}

				if c.dcDoNotInferLinks {
					dockerDeployTarget = dockerDeployTarget.WithInferLinks(false)
				}

				m = m.WithDeployTarget(dockerDeployTarget)
			}

			if len(c.dcPortBindings) > 0 && !m.IsDC() {
				m = m.WithDeployTarget(model.DockerComposeTarget{})
			}

			mt := newManifestTargetWithLoadBalancerURLs(m, c.lbURLs)
			if len(c.dcPortBindings) > 0 {
				dcState := mt.State.DCRuntimeState()
				dcState.Ports = c.dcPortBindings
				mt.State.RuntimeState = dcState
			}
			actual := ManifestTargetEndpoints(mt)
			assertLinks(t, c.expected, actual)
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

// assert.Equal on a URL is ugly and hard to read; where it's helpful, compare URLs as strings
func assertLinks(t *testing.T, expected, actual []model.Link) {
	require.Len(t, actual, len(expected), "expected %d links but got %d", len(expected), len(actual))
	expectedStrs := model.LinksToURLStrings(expected)
	actualStrs := model.LinksToURLStrings(actual)
	// compare the URLs as strings for readability
	if assert.Equal(t, expectedStrs, actualStrs, "url string comparison") {
		// and if those match, compare everything else
		assert.Equal(t, expected, actual)
	}
}

func k8sManifest(t testing.TB, name model.ManifestName, yaml string) model.Manifest {
	t.Helper()
	kt, err := k8s.NewTargetForYAML(name.TargetName(), yaml, nil)
	require.NoError(t, err, "Failed to create Kubernetes deploy target")
	return model.Manifest{Name: name}.WithDeployTarget(kt)
}

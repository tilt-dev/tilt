package dockercomposelogstream

import (
	"strings"
	"testing"
	"time"

	typescontainer "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// Make sure we stream logs correctly when
// we're triggered by a project event.
func TestBasicLogsFromEvents(t *testing.T) {
	f := newFixture(t)

	output := make(chan string, 1)
	defer close(output)

	containerID := "my-container-id"
	f.dc.ContainerLogChans[containerID] = output
	f.dcc.ContainerIDDefault = container.ID(containerID)

	obj := v1alpha1.DockerComposeLogStream{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fe",
		},
		Spec: v1alpha1.DockerComposeLogStreamSpec{
			Service: "fe",
			Project: v1alpha1.DockerComposeProject{
				YAML: "fake-yaml",
			},
		},
	}
	f.Create(&obj)

	container := typescontainer.State{
		Status:    "running",
		Running:   true,
		StartedAt: "2021-09-08T19:58:01.483005100Z",
	}
	f.dc.Containers[containerID] = container

	event := dockercompose.Event{Type: dockercompose.TypeContainer, ID: containerID, Service: "fe"}
	f.dcc.SendEvent(event)

	expected := "hello world"
	output <- expected

	assert.Eventually(t, func() bool {
		return strings.Contains(f.Stdout(), expected)
	}, time.Second, 10*time.Millisecond)
}

// Make sure we stream logs correctly when
// we're connecting to an existing project.
func TestBasicLogsFromExisting(t *testing.T) {
	f := newFixture(t)

	output := make(chan string, 1)
	defer close(output)

	containerID := "my-container-id"
	f.dc.ContainerLogChans[containerID] = output
	c := typescontainer.State{
		Status:    "running",
		Running:   true,
		StartedAt: "2021-09-08T19:58:01.483005100Z",
	}
	f.dc.Containers[containerID] = c
	f.dcc.ContainerIDDefault = container.ID(containerID)

	obj := v1alpha1.DockerComposeLogStream{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fe",
		},
		Spec: v1alpha1.DockerComposeLogStreamSpec{
			Service: "fe",
			Project: v1alpha1.DockerComposeProject{
				YAML: "fake-yaml",
			},
		},
	}
	f.Create(&obj)

	expected := "hello world"
	output <- expected

	assert.Eventually(t, func() bool {
		return strings.Contains(f.Stdout(), expected)
	}, time.Second, 10*time.Millisecond)
}

func TestTwoServices(t *testing.T) {
	f := newFixture(t)

	feContainerID := "fe-id"
	beContainerID := "be-id"

	feOutput := make(chan string, 1)
	defer close(feOutput)
	f.dc.ContainerLogChans[feContainerID] = feOutput
	f.dcc.ContainerIDByService["fe"] = container.ID(feContainerID)

	beOutput := make(chan string, 1)
	defer close(beOutput)
	f.dc.ContainerLogChans[beContainerID] = beOutput
	f.dcc.ContainerIDByService["be"] = container.ID(beContainerID)

	project := v1alpha1.DockerComposeProject{
		YAML: "fake-yaml",
	}

	fe := v1alpha1.DockerComposeLogStream{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fe",
		},
		Spec: v1alpha1.DockerComposeLogStreamSpec{
			Service: "fe",
			Project: project,
		},
	}
	f.Create(&fe)

	be := v1alpha1.DockerComposeLogStream{
		ObjectMeta: metav1.ObjectMeta{
			Name: "be",
		},
		Spec: v1alpha1.DockerComposeLogStreamSpec{
			Service: "be",
			Project: project,
		},
	}

	container := typescontainer.State{
		Status:    "running",
		Running:   true,
		StartedAt: "2021-09-08T19:58:01.483005100Z",
	}
	f.dc.Containers[feContainerID] = container
	f.dc.Containers[beContainerID] = container

	f.dcc.SendEvent(dockercompose.Event{Type: dockercompose.TypeContainer, ID: feContainerID, Service: "fe"})
	f.dcc.SendEvent(dockercompose.Event{Type: dockercompose.TypeContainer, ID: beContainerID, Service: "be"})

	// Create the BE log stream later, and make sure everything's OK.
	f.Create(&be)
	expected := "hello backend"
	beOutput <- expected

	assert.Eventually(t, func() bool {
		return strings.Contains(f.Stdout(), expected)
	}, time.Second, 10*time.Millisecond)
}

type fixture struct {
	*fake.ControllerFixture
	r   *Reconciler
	dc  *docker.FakeClient
	dcc *dockercompose.FakeDCClient
}

func newFixture(t *testing.T) *fixture {
	cfb := fake.NewControllerFixtureBuilder(t)
	dcCli := dockercompose.NewFakeDockerComposeClient(t, cfb.Context())
	dCli := docker.NewFakeClient()
	r := NewReconciler(cfb.Client, cfb.Store, dcCli, dCli)

	return &fixture{
		ControllerFixture: cfb.WithRequeuer(r.requeuer).Build(r),
		r:                 r,
		dc:                dCli,
		dcc:               dcCli,
	}
}

package cluster

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s"
)

func TestConnectionManager(t *testing.T) {
	type tc struct {
		input             *connection
		expectedK8sErr    string
		expectedDockerErr string
	}

	fakeK8s := k8s.NewFakeK8sClient(t)
	fakeDocker := docker.NewFakeClient()

	tcs := []tc{
		{
			input: &connection{
				connType:  connectionTypeK8s,
				k8sClient: fakeK8s,
			},
			expectedDockerErr: "incorrect cluster client type: got kubernetes, expected docker",
		},
		{
			input: &connection{
				connType:  connectionTypeK8s,
				k8sClient: fakeK8s,
				error:     "connection error",
			},
			expectedK8sErr:    "connection error",
			expectedDockerErr: "incorrect cluster client type: got kubernetes, expected docker",
		},
		{
			input:             nil,
			expectedK8sErr:    NotFoundError.Error(),
			expectedDockerErr: NotFoundError.Error(),
		},
		{
			input: &connection{
				connType:     connectionTypeDocker,
				dockerClient: fakeDocker,
			},
			expectedK8sErr: "incorrect cluster client type: got docker, expected kubernetes",
		},
		{
			input: &connection{
				connType:     connectionTypeDocker,
				dockerClient: fakeDocker,
				error:        "some docker error",
			},
			expectedK8sErr:    "incorrect cluster client type: got docker, expected kubernetes",
			expectedDockerErr: "some docker error",
		},
	}

	for i := range tcs {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			cm := NewConnectionManager()
			nn := types.NamespacedName{Namespace: "foo", Name: "bar"}
			if tcs[i].input != nil {
				cm.store(nn, *tcs[i].input)
			}

			kCli, err := cm.GetK8sClient(nn)
			if tcs[i].expectedK8sErr != "" {
				if assert.EqualError(t, err, tcs[i].expectedK8sErr) {
					assert.Nil(t, kCli, "K8sClient should be nil on error")
				}
			} else {
				if assert.NoError(t, err, "Unexpected error getting K8sClient") {
					assert.NotNil(t, kCli, "K8sClient should not be nil when no error")
				}
			}

			dCli, err := cm.GetComposeDockerClient(nn)
			if tcs[i].expectedDockerErr != "" {
				if assert.EqualError(t, err, tcs[i].expectedDockerErr) {
					assert.Nil(t, dCli, "DockerClient should be nil on error")
				}
			} else {
				if assert.NoError(t, err, "Unexpected error getting DockerClient") {
					assert.NotNil(t, dCli, "DockerClient should not be nil when no error")
				}
			}
		})
	}
}

package docker

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/clusterid"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/testutils"
)

type EnvTest struct {
	product     clusterid.Product
	kubecontext string
	dockerHost  string
	warning     string
	match       bool
}

type fakeDaemonClient struct {
	host string
}

func (f *fakeDaemonClient) DaemonHost() string {
	return f.host
}

func TestColimaEnv(t *testing.T) {

	table := []EnvTest{
		{
			product:     clusterid.ProductColima,
			kubecontext: "colima",
			dockerHost:  "unix://~/.colima/docker.sock",
			match:       true,
		},
		{
			product:     clusterid.ProductColima,
			kubecontext: "colima",
			dockerHost:  "unix://~/.colima/default/docker.sock",
			match:       true,
		},
		{
			product:     clusterid.ProductColima,
			kubecontext: "colima-test",
			dockerHost:  "unix://~/.colima-test/docker.sock",
			match:       true,
		},
		{
			product:     clusterid.ProductColima,
			kubecontext: "colima-test",
			dockerHost:  "unix://~/.colima/test/docker.sock",
			match:       true,
		},
		{
			product:     clusterid.ProductColima,
			kubecontext: "colima-test",
			dockerHost:  "unix://~/.colima/docker.sock",
			match:       false,
			warning:     "connected to Kubernetes on Colima profile test, but building on Docker on Colima profile default",
		},
		{
			product:     clusterid.ProductColima,
			kubecontext: "colima",
			dockerHost:  "unix://~/.colima-test/docker.sock",
			match:       false,
			warning:     "connected to Kubernetes on Colima profile default, but building on Docker on Colima profile test",
		},
		{
			product:     clusterid.ProductColima,
			kubecontext: "colima-test",
			dockerHost:  "unix://~/.docker/desktop/docker.sock",
			match:       false,
			warning:     "connected to Kubernetes running on Colima, but building on a non-Colima Docker socket",
		},
		{
			product:     clusterid.ProductOrbstack,
			kubecontext: "orbstack",
			dockerHost:  "unix://~/.orbstack/run/docker.sock",
			match:       true,
		},
		{
			product:     clusterid.ProductOrbstack,
			kubecontext: "orbstack",
			dockerHost:  "unix://~/.docker/desktop/docker.sock",
			match:       false,
			warning:     "connected to Kubernetes running on Orbstack, but building on a non-Orbstack Docker socket",
		},
	}

	for i, c := range table {
		t.Run(fmt.Sprintf("env-%d", i), func(t *testing.T) {
			out := &bytes.Buffer{}
			ctx, _, _ := testutils.ForkedCtxAndAnalyticsForTest(out)
			env := Env{
				Client: &fakeDaemonClient{
					host: c.dockerHost,
				},
			}

			result := willBuildToKubeContext(ctx, c.product, k8s.KubeContext(c.kubecontext), env)
			assert.Equal(t, result, c.match)
			if c.warning != "" {
				assert.Contains(t, out.String(), c.warning)
			} else {
				assert.Equal(t, out.String(), c.warning)
			}
		})
	}
}

//go:build integration
// +build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTooManyDeployments(t *testing.T) {
	f := newK8sFixture(t, "too_many_deployments")

	f.runCommandSilently("kubectl", "apply", "-f", "namespace.yaml")
	f.runCommandSilently("kubectl", "apply", "-f", "too_many_deployments/deployments.yaml")

	// Use tilt to verify that busybox1 ONLY is up
	f.TiltCI("busybox1")

	// Run tilt ci again when we know services are running.
	f.TiltCI("busybox1")

	// Make sure we're not getting throttled by the kubernetes client.
	assert.NotContains(t, f.logs.String(), "Throttling request")
}

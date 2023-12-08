//go:build integration
// +build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
)

func TestJobReattach(t *testing.T) {
	f := newK8sFixture(t, "job_reattach")
	f.SetRestrictedCredentials()

	f.TiltCI()

	_, _, podNames := f.AllPodsInPhase(f.ctx, "app=job-reattach-db-init", v1.PodSucceeded)
	require.Equal(t, 1, len(podNames))

	f.runCommandSilently("kubectl", "delete", "-n", "tilt-integration", "pod", podNames[0])

	// Make sure 'ci' still succeeds, but we don't restart the Job pod.
	f.TiltCI()

	_, _, podNames = f.AllPodsInPhase(f.ctx, "app=job-reattach-db-init", v1.PodSucceeded)
	require.Equal(t, 0, len(podNames))
}

//go:build integration
// +build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
)

func TestJobFail(t *testing.T) {
	f := newK8sFixture(t, "job_fail")
	f.SetRestrictedCredentials()

	// Make sure 'ci' fails.
	err := f.tilt.CI(f.ctx, f.LogWriter())
	require.Error(t, err)
	assert.Contains(t, f.logs.String(), "db-init job failed")

	_, _, podNames := f.AllPodsInPhase(f.ctx, "app=job-fail-db-init", v1.PodFailed)
	require.Equal(t, 1, len(podNames))
}

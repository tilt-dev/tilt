//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

func TestShortlivedPods(t *testing.T) {
	f := newK8sFixture(t, "shortlived_pods")
	defer f.TearDown()

	f.TiltUp()

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.WaitForAllPodsInPhase(ctx, "app=shortlived-pod-fail", v1.PodFailed)
	f.WaitForAllPodsInPhase(ctx, "app=shortlived-pod-success", v1.PodSucceeded)

	out := f.logs.String()
	assert.Contains(t, out, "this is a successful job")
	assert.Contains(t, out, "this job will fail")
}

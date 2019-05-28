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

	f.TiltWatch()

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.WaitForAllPodsInPhase(ctx, "app=shortlived-pod", []v1.PodPhase{v1.PodSucceeded})
	f.KillProcs()

	assert.Contains(t, f.logs.String(), "this is a successful job")
}

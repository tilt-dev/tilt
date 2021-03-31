//+build integration

package integration

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func getNodeName(f *k8sFixture) string {
	cmd := exec.Command("kubectl", "get", "nodes", "-o", "jsonpath={.items[*].metadata.name}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		f.t.Fatal(errors.Wrap(err, "get nodes"))
	}

	nodeName := strings.TrimSpace(string(out))
	assert.NotEqual(f.t, "", nodeName)
	return nodeName
}

func markNodeUnschedulable(f *k8sFixture, name string) {
	f.runOrFail(
		exec.Command("kubectl", "taint", "nodes", name, "key=value:NoSchedule", "--overwrite"),
		"markNodeUnschedulable")
}

func markNodeSchedulable(f *k8sFixture, name string) {
	// There is no idempotent way to remove a taint.
	// If the taint doesn't exist, removing the taint will fail. This is dumb.
	// But you can use --overwrite to add a taint idempotently, then remove it :eyeroll:
	markNodeUnschedulable(f, name)
	f.runOrFail(
		exec.Command("kubectl", "taint", "nodes", name, "key:NoSchedule-"),
		"markNodeSchedulable")
}

// TestUnschedulableEvent is testing two things:
//	1) Important K8s events (such as unschedulable pods) are propagated to logs (at least until #3532 is done)
//	2) Unschedulable pods continue to be monitored by Tilt after the situation is resolved
//		(see comments on assertions - this is hacky right now!)
func TestUnschedulableEvent(t *testing.T) {
	f := newK8sFixture(t, "event")
	defer f.TearDown()

	node := getNodeName(f)
	markNodeUnschedulable(f, node)
	defer markNodeSchedulable(f, node)

	f.TiltUp()

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.WaitUntil(ctx, "unschedulable pod event", func() (string, error) {
		logs := strings.Split(f.logs.String(), "\n")
		for _, log := range logs {
			if strings.Contains(log, "K8s EVENT") && strings.Contains(log, "the pod didn't tolerate") {
				return "unschedulable event", nil
			}
		}

		return "", nil
	}, "unschedulable event")

	markNodeSchedulable(f, node)

	// Make sure that the pod schedules successfully
	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "Hello world")

	// TODO(milas): once Tilt API is more fully-fledged, assert on the status of resource (from Tilt's perspective)
	// 	to verify that it's failing while the node is unschedulable and replace this log hack with a corresponding
	// 	status check to show it's passing as well (the log inspection is being used as a way to check that Tilt is
	// 	still correctly watching the pod even though it was temporarily in an error state)
	assert.NoError(t, f.logs.WaitUntilContains("Starting HTTP server", 2*time.Second))
}

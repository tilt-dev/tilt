//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDisableK8s(t *testing.T) {
	f := newK8sFixture(t, "disable")

	f.TiltUp()

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.WaitForAllPodsReady(ctx, "app=disabletest")

	setDisabled(f.fixture, "disabletest", true)

	f.WaitUntil(ctx, "pod gone", func() (string, error) {
		out, err := f.runCommand("kubectl", "get", "pod", namespaceFlag, "-lapp=disabletest", "--no-headers")
		return out.String(), err
	}, "No resources found")

	setDisabled(f.fixture, "disabletest", false)

	f.WaitForAllPodsReady(ctx, "app=disabletest")
}

func TestDisableDC(t *testing.T) {
	f := newDCFixture(t, "disable")

	f.dockerKillAll("tilt")
	f.TiltUp("-f", "Tiltfile.dc")

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()

	f.WaitUntil(ctx, "uiresource available", func() (string, error) {
		out, err := f.tilt.Get(ctx, "uiresource")
		return string(out), err
	}, "disabletest")

	f.runOrFail(f.tilt.cmd(ctx, []string{"wait", "--for=condition=Ready", "uiresource/disabletest"}, nil), "wait")

	psArgs := []string{
		"ps", "-f", "name=disabletest", "--format", "{{.Image}}",
	}

	out, err := f.dockerCmdOutput(psArgs)
	require.NoError(t, err)
	require.Contains(t, out, "disabletest")

	f.WaitUntil(ctx, "disable configmap available", func() (string, error) {
		out, err := f.tilt.Get(ctx, "configmap")
		return string(out), err
	}, "disabletest-disable")

	setDisabled(f.fixture, "disabletest", true)

	require.Eventually(t, func() bool {
		out, _ := f.dockerCmdOutput(psArgs)
		return len(out) == 0
	}, time.Minute, 15*time.Millisecond, "dc service stopped")

	setDisabled(f.fixture, "disabletest", false)

	f.WaitUntil(ctx, "service up", func() (string, error) {
		return f.dockerCmdOutput(psArgs)
	}, "disabletest")
}

func setDisabled(f *fixture, resourceName string, isDisabled bool) {
	err := f.tilt.Patch(
		f.ctx,
		"configmap",
		fmt.Sprintf("{\"data\": {\"isDisabled\": \"%s\"}}", strconv.FormatBool(isDisabled)),
		fmt.Sprintf("%s-disable", resourceName),
	)

	require.NoErrorf(f.t, err, "setting disable state for %s to %v", resourceName, isDisabled)
}

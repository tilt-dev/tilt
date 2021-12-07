//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDisable(t *testing.T) {
	f := newK8sFixture(t, "disable")
	defer f.TearDown()

	f.TiltUp()

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.WaitForAllPodsReady(ctx, "app=disabletest")

	setDisabled(f, "disabletest", true)

	f.WaitUntil(ctx, "pod gone", func() (string, error) {
		out, err := f.runCommand("kubectl", "get", "pod", namespaceFlag, "-lapp=disabletest", "--no-headers")
		return out.String(), err
	}, "No resources found")

	setDisabled(f, "disabletest", false)

	f.WaitForAllPodsReady(ctx, "app=disabletest")
}

func setDisabled(f *k8sFixture, resourceName string, isDisabled bool) {
	out, err := f.runCommand(
		"tilt",
		"--port",
		fmt.Sprintf("%d", f.tilt.port),
		"patch",
		"configmap",
		"-p",
		fmt.Sprintf("{\"data\": {\"isDisabled\": \"%s\"}}", strconv.FormatBool(isDisabled)),
		"--",
		fmt.Sprintf("%s-disable", resourceName))
	if !assert.NoError(f.t, err) {
		f.t.Fatalf("setting service disable state failed: %s", out.String())
	}
}

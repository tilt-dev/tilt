//+build integration

package integration

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Make sure that Tilt crashes if there are two tilts running
func TestCrash(t *testing.T) {
	f := newK8sFixture(t, "oneup")
	defer f.TearDown()

	f.TiltWatch()
	time.Sleep(500 * time.Millisecond)

	out := bytes.NewBuffer(nil)
	_ = f.tiltCmd([]string{"up", "--watch=false", "--hud=false"}, out).Run()
	assert.Contains(t, out.String(), "Cannot start Tilt")
}

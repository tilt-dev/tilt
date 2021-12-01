//go:build integration
// +build integration

package integration

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Make sure that Tilt crashes if there are two tilts running on the same requested port.
func TestCrash(t *testing.T) {
	f := newK8sFixture(t, "oneup")
	defer f.TearDown()

	f.TiltUp("--port=9975")
	time.Sleep(500 * time.Millisecond)

	out := bytes.NewBuffer(nil)
	res, err := f.tilt.Up(f.ctx, UpCommandUp, out, "--port=9975")
	assert.NoError(t, err)
	<-res.Done()
	assert.Contains(t, out.String(), "Tilt cannot start")
	assert.NotContains(t, out.String(), "Usage:")
}

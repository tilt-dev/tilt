//+build integration

package integration

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Make sure that Tilt crashes if there are two tilts running on the same port.
func TestCrash(t *testing.T) {
	f := newK8sFixture(t, "oneup")
	defer f.TearDown()

	f.TiltUp()
	time.Sleep(500 * time.Millisecond)

	out := bytes.NewBuffer(nil)
	// fixture assigns a random unused port when created, which is used for all Tilt commands, so this
	// will collide with the previous invocation
	res, err := f.tilt.Up(out)
	assert.NoError(t, err)
	<-res.Done()
	assert.Contains(t, out.String(), "Tilt cannot start")
	assert.NotContains(t, out.String(), "Usage:")
}

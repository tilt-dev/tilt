//+build integration

package integration

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Make sure that Tilt crashes if there are two tilts running on the same port.
func TestCrash(t *testing.T) {
	f := newK8sFixture(t, "oneup")
	defer f.TearDown()

	f.TiltUp()
	require.NotZero(t, f.activeTiltUp.port)
	time.Sleep(500 * time.Millisecond)

	out := bytes.NewBuffer(nil)
	// explicitly pass a port argument or the integration tests will pick a random unused one, thus defeating
	// the point of the test
	res, err := f.tilt.Up(out, fmt.Sprintf("--port=%d", f.activeTiltUp.port))
	assert.NoError(t, err)
	<-res.Done()
	assert.Contains(t, out.String(), "Tilt cannot start")
	assert.NotContains(t, out.String(), "Usage:")
}

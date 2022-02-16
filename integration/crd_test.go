//go:build integration
// +build integration

package integration

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCRD(t *testing.T) {
	f := newK8sFixture(t, "crd")

	f.TiltUp()

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()

	f.WaitUntil(ctx, "Waiting for UM to show up", func() (string, error) {
		out, _ := f.runCommand("kubectl", "get", "um", namespaceFlag)
		return out.String(), nil
	}, "bobo")

	out, err := f.runCommand("kubectl", "get", "um", namespaceFlag, "-o=yaml")
	require.NoError(t, err)
	contents := out.String()

	// Make sure image injection didn't replace the name
	// or the non-image field, but did replace the image field.
	assert.Contains(t, contents, "name: bobo\n")
	assert.Contains(t, contents, "nonImage: bobo\n")
	assert.NotContains(t, contents, "image: bobo\n")
	assert.Regexp(t, regexp.MustCompile("imagePullPolicy: (IfNotPresent|Never)\n"), contents)
}

// Make sure that running 'tilt down' with no resources installed
// yet handles 'not found' correctly
func TestCRDNotFound(t *testing.T) {
	f := newK8sFixture(t, "crd")

	err := f.tilt.Down(f.ctx, ioutil.Discard)
	require.NoError(t, err)
}

// Make sure that running 'tilt down' tears down the CRD
// even when the CR doesn't exist.
func TestCRDPartialNotFound(t *testing.T) {
	f := newK8sFixture(t, "crd")

	out, err := f.runCommand("kubectl", "apply", "-f", filepath.Join(f.dir, "crd.yaml"))
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "uselessmachines.tilt.dev created")

	_, err = f.runCommand("kubectl", "get", "crd", "uselessmachines.tilt.dev")
	assert.NoError(t, err)

	err = f.tilt.Down(f.ctx, ioutil.Discard)
	require.NoError(t, err)

	// Make sure the crds were deleted.
	out, err = f.runCommand("kubectl", "get", "crd", "uselessmachines.tilt.dev")
	if assert.Error(t, err) {
		assert.Contains(t, out.String(), `"uselessmachines.tilt.dev" not found`)
	}
}

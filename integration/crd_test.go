//+build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCRD(t *testing.T) {
	f := newK8sFixture(t, "crd")
	defer f.TearDown()

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
	assert.Contains(t, contents, "imagePullPolicy: IfNotPresent\n")
}

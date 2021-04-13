package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIResources(t *testing.T) {
	f := newServerFixture(t)
	defer f.TearDown()

	out := bytes.NewBuffer(nil)
	cmd := newApiresourcesCmd()
	cmd.register()
	cmd.options.IOStreams.Out = out

	err := cmd.run(f.ctx, nil)
	require.NoError(t, err)

	assert.Contains(t, out.String(),
		`filewatches                  tilt.dev/v1alpha1   false        FileWatch`)
}

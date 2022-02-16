package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
)

func TestTiltfileResult(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	f.Chdir()

	f.WriteFile("Tiltfile", `

v1alpha1.extension_repo(name='default', url='https://github.com/tilt-dev/tilt-extensions')
local_resource(name='hi', cmd='echo hi', serve_cmd='echo bye')
`)

	out := bytes.NewBuffer(nil)
	errOut := bytes.NewBuffer(nil)
	cmd := newTiltfileResultCmd()
	cmd.streams.Out = out
	cmd.streams.ErrOut = errOut
	cmd.fileName = "Tiltfile"
	cmd.exit = func(x int) {}

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	err := cmd.run(ctx, nil)
	require.NoError(t, err)

	assert.Contains(t, out.String(), `"Error": null`)
	assert.Contains(t, out.String(), `"Name": "hi"`)
	assert.Contains(t, out.String(), `"url": "https://github.com/tilt-dev/tilt-extensions"`)
}

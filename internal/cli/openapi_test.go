package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tilt-dev/tilt/internal/testutils"
)

func TestOpenapi(t *testing.T) {
	out := bytes.NewBuffer(nil)
	streams := genericclioptions.IOStreams{Out: out}

	cmd := newOpenapiCmd(streams)
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	err := cmd.run(ctx, nil)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(out.String(), `{
  "swagger": "2.0",
  "info": {
    "title": "tilt",`))
}

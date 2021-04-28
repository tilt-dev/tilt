package cli

import (
	"bytes"
	"regexp"
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

	// output spacing is dependent on longest resource name, so a regex is used to keep this test
	// from being overly brittle
	outputRe := regexp.MustCompile(`filewatches[ ]+tilt.dev/v1alpha1[ ]+false[ ]+FileWatch`)

	cmdOutput := out.String()
	assert.Truef(t, outputRe.MatchString(cmdOutput),
		"Command output was not as expected. Output:\n%s\n", cmdOutput)
}

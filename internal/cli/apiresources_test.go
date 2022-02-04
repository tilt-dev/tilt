package cli

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func TestAPIResources(t *testing.T) {
	f := newServerFixture(t)
	defer f.TearDown()

	streams, _, out, _ := genericclioptions.NewTestIOStreams()
	cmd := newApiresourcesCmd(streams)
	cmd.register()

	err := cmd.run(f.ctx, nil)
	require.NoError(t, err)

	// output spacing is dependent on longest resource name, so a regex is used to keep this test
	// from being overly brittle
	outputRe := regexp.MustCompile(`filewatches[ ]+fw[ ]+tilt.dev/v1alpha1[ ]+false[ ]+FileWatch`)

	cmdOutput := out.String()
	assert.Truef(t, outputRe.MatchString(cmdOutput),
		"Command output was not as expected. Output:\n%s\n", cmdOutput)
}

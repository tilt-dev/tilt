//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigMap(t *testing.T) {
	f := newK8sFixture(t, "configmap")

	f.TiltUp()

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()

	f.WaitUntil(ctx, "Waiting for small configmap to show up", func() (string, error) {
		out, _ := f.runCommand("kubectl", "get", "configmap", "small-configmap", namespaceFlag, "-o=go-template", "--template='{{.data}}'")
		return out.String(), nil
	}, "hello world")

	firstCreationTime, err := f.runCommand("kubectl", "get", "configmap", "small-configmap", namespaceFlag, "-o=go-template", "--template='{{.metadata.creationTimestamp}}'")
	require.NoError(t, err)
	require.NotEqual(t, "", firstCreationTime.String())

	f.ReplaceContents("small.txt", "hello world", "goodbye world")

	f.WaitUntil(ctx, "Waiting for small configmap to get replaced", func() (string, error) {
		out, _ := f.runCommand("kubectl", "get", "configmap", "small-configmap", namespaceFlag, "-o=go-template", "--template='{{.data}}'")
		return out.String(), nil
	}, "goodbye world")

	secondCreationTime, err := f.runCommand("kubectl", "get", "configmap", "small-configmap", namespaceFlag, "-o=go-template", "--template='{{.metadata.creationTimestamp}}'")
	require.NoError(t, err)
	require.NotEqual(t, "", secondCreationTime.String())

	// Make sure we applied the configmap instead of recreating it
	assert.Equal(t, firstCreationTime, secondCreationTime)
}

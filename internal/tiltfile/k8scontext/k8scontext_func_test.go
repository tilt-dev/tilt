package k8scontext

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/clusterid"
)

func TestCacheInvalidationMechanism(t *testing.T) {
	// Test the cache invalidation mechanism directly
	var invalidatedKey types.NamespacedName
	var invalidationCalled bool

	// Set up a mock cache invalidator
	SetGlobalCacheInvalidator(func(key types.NamespacedName) {
		invalidatedKey = key
		invalidationCalled = true
	})

	// Call InvalidateCache directly
	testKey := types.NamespacedName{Name: "test-cluster", Namespace: "test-ns"}
	InvalidateCache(testKey)

	// Check that cache invalidation was called
	assert.True(t, invalidationCalled, "Cache invalidation should have been called")
	assert.Equal(t, testKey, invalidatedKey, "Should have received the correct key")
}

func TestK8sContextFunctionExists(t *testing.T) {
	// Test that the k8s_context function exists and can be called
	f := NewFixture(t, "docker-desktop", "default", clusterid.ProductDockerDesktop)
	f.File("Tiltfile", `
# Test that the function exists and returns something
result = k8s_context("docker-desktop")
print("Result:", result)
`)

	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)

	// Just verify that it ran without error and produced output
	output := f.PrintOutput()
	assert.Contains(t, output, "Result:", "Function should have produced output")
}

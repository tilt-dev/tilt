//go:build integration
// +build integration

package integration

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The Docker CLI library loads all env variables on it.
// So the only way to really test if we're loading them properly
// is to run tilt.
func TestEnvInit(t *testing.T) {
	t.Setenv("DOCKER_TLS_VERIFY", "1")
	t.Setenv("DOCKER_CERT_PATH", "/tmp/unused-path")

	f := newK8sFixture(t, "oneup")
	out := &bytes.Buffer{}
	err := f.tilt.CI(f.ctx, out)
	assert.Error(t, err)
	assert.Contains(t, out.String(), "unable to resolve docker endpoint: open /tmp/unused-path/ca.pem")
}

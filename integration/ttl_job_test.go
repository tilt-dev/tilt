//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// Create a job with TTL cleanup.
// https://github.com/tilt-dev/tilt/issues/5949
func TestTTLJob(t *testing.T) {
	f := newK8sFixture(t, "ttl_job")

	f.TiltUp()

	ctx, cancel := context.WithTimeout(f.ctx, 30*time.Second)
	defer cancel()

	f.WaitUntil(ctx, "failed to run job", func() (string, error) {
		return f.logs.String(), nil
	}, "ttl-job │ job-success")

	f.WaitUntil(ctx, "failed to run local_resource", func() (string, error) {
		return f.logs.String(), nil
	}, "local │ success")

	out, err := f.tilt.Get(ctx, "uiresource", "ttl-job")
	assert.NoError(t, err)

	log.Println(string(out))

	var resource v1alpha1.UIResource
	err = json.NewDecoder(bytes.NewBuffer(out)).Decode(&resource)
	assert.NoError(t, err)
	assert.Equal(t, "ok", string(resource.Status.RuntimeStatus))
}

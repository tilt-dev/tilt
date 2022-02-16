//go:build integration
// +build integration

package integration

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTiltDemo(t *testing.T) {
	f := newFixture(t, "")
	defer func() {
		if !f.skipTiltDown {
			// chdir to the sample project directory so that we can properly run a tilt down
			// (this only happens if we did NOT make a cluster that got torn down so that we don't leave
			// behind pods)
			m, err := filepath.Glob(filepath.Join(f.dir, "*", "github.com", "tilt-dev", "tilt-avatars"))
			if err != nil || len(m) != 1 {
				t.Errorf("Could not find sample project: %v", err)
			}
			if err := os.Chdir(m[0]); err != nil {
				t.Errorf("Could not chdir to sample project dir (%q): %v", m[0], err)
			}
		}
	}()

	var extraArgs []string
	if dockerHost := os.Getenv("DOCKER_HOST"); dockerHost != "" {
		// this is heavy-handed but sufficient for CI purposes
		t.Logf("Detected non-default Docker configuration, will not attempt to create ephemeral K8s cluster (DOCKER_HOST=%s)", dockerHost)
		extraArgs = append(extraArgs, "--no-cluster")
		extraArgs = append(extraArgs, "--tmpdir", f.dir)
	} else {
		// we don't need to run tilt down because the entire temporary cluster will be destroyed
		f.skipTiltDown = true
	}

	f.TiltDemo(extraArgs...)

	ctx, cancel := context.WithTimeout(f.ctx, 3*time.Minute)
	defer cancel()

	f.CurlUntilStatusCode(ctx, "http://localhost:5734/ready", http.StatusNoContent)
}

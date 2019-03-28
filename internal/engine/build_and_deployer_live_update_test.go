package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/k8s"
)

func TestLiveUpdateLocalContainerUpdate(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, f.deployInfo())
	manifest := NewSanchoDockerBuildManifestWithLiveUpdate(f)
	targets := buildTargets(manifest)
	result, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
	if err != nil {
		t.Fatal(err)
	}

	if f.docker.BuildCount != 0 {
		t.Errorf("Expected no docker build, actual: %d", f.docker.BuildCount)
	}
	if f.docker.PushCount != 0 {
		t.Errorf("Expected no push to docker, actual: %d", f.docker.PushCount)
	}
	if f.docker.CopyCount != 1 {
		t.Errorf("Expected 1 copy to docker container call, actual: %d", f.docker.CopyCount)
	}
	if len(f.docker.ExecCalls) != 1 {
		t.Errorf("Expected 1 exec in container call, actual: %d", len(f.docker.ExecCalls))
	}
	f.assertContainerRestarts(1)

	id := manifest.ImageTargetAt(0).ID()
	_, hasResult := result[id]
	assert.True(t, hasResult)
	assert.Equal(t, k8s.MagicTestContainerID, result.OneAndOnlyContainerID().String())
}

func TestCustomBuildLiveUpdateLocalContainerUpdate(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(alreadyBuiltSet, []string{changed}, f.deployInfo())
	manifest := NewSanchoCustomBuildManifestWithLiveUpdate(f)
	targets := buildTargets(manifest)
	result, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
	if err != nil {
		t.Fatal(err)
	}

	if f.docker.BuildCount != 0 {
		t.Errorf("Expected no docker build, actual: %d", f.docker.BuildCount)
	}
	if f.docker.PushCount != 0 {
		t.Errorf("Expected no push to docker, actual: %d", f.docker.PushCount)
	}
	if f.docker.CopyCount != 1 {
		t.Errorf("Expected 1 copy to docker container call, actual: %d", f.docker.CopyCount)
	}
	if len(f.docker.ExecCalls) != 1 {
		t.Errorf("Expected 1 exec in container call, actual: %d", len(f.docker.ExecCalls))
	}
	f.assertContainerRestarts(1)

	id := manifest.ImageTargetAt(0).ID()
	_, hasResult := result[id]
	assert.True(t, hasResult)
	assert.Equal(t, k8s.MagicTestContainerID, result.OneAndOnlyContainerID().String())
}

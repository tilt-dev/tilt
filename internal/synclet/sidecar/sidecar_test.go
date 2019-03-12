package sidecar

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
)

func TestInjectSyncletSidecar(t *testing.T) {
	entities, err := k8s.ParseYAMLFromString(testyaml.SanchoYAML)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, len(entities))
	entity := entities[0]
	selector := container.MustParseSelector("gcr.io/some-project-162817/sancho")
	newEntity, replaced, err := InjectSyncletSidecar(entity, selector)
	if err != nil {
		t.Fatal(err)
	} else if !replaced {
		t.Errorf("Expected replacement in:\n%s", testyaml.SanchoYAML)
	}

	result, err := k8s.SerializeYAML([]k8s.K8sEntity{newEntity})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, SyncletImageName) {
		t.Errorf("could not find image in yaml (%s):\n%s", SyncletImageName, result)
	}
}

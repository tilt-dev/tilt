package k8s

import (
	"fmt"

	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/value"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Deserializing pod readiness from starlark values.
type PodReadinessMode struct {
	Value model.PodReadinessMode
}

func (m *PodReadinessMode) Unpack(v starlark.Value) error {
	s, ok := value.AsString(v)
	if !ok {
		return fmt.Errorf("Must be a string. Got: %s", v.Type())
	}

	for _, mode := range []model.PodReadinessMode{
		model.PodReadinessIgnore,
		model.PodReadinessWait,
		model.PodReadinessSucceeded,
		model.PodReadinessNone,
	} {
		if s == string(mode) {
			m.Value = mode
			return nil
		}
	}

	return fmt.Errorf("Invalid value. Allowed: {%s, %s}. Got: %s", model.PodReadinessIgnore, model.PodReadinessWait, s)
}

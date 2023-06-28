package k8s

import (
	"fmt"
	"strings"

	"go.starlark.net/starlark"
	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/k8s"
)

const fmtDuplicateYAMLDetectedError = `Duplicate YAML: %s
Ignore this error with k8s_yaml(..., allow_duplicates=True)
YAML originally registered at: %s`

func DuplicateYAMLDetectedError(id, stackTrace string) error {
	indentedTrace := strings.Join(strings.Split(stackTrace, "\n"), "\n  ")
	return fmt.Errorf(fmtDuplicateYAMLDetectedError, id, indentedTrace)
}

type ObjectSpec struct {
	// The resource spec
	Entity k8s.K8sEntity

	// The stack trace where this resource was registered.
	// Helpful for reporting duplicates.
	StackTrace string
}

// Keeps track of all the Kubernetes objects registered during Tiltfile Execution.
type State struct {
	ObjectSpecRefs  []v1.ObjectReference
	ObjectSpecIndex map[v1.ObjectReference]ObjectSpec
}

func NewState() *State {
	return &State{
		ObjectSpecIndex: make(map[v1.ObjectReference]ObjectSpec),
	}
}

func (s *State) Entities() []k8s.K8sEntity {
	result := make([]k8s.K8sEntity, len(s.ObjectSpecIndex))
	for i, ref := range s.ObjectSpecRefs {
		result[i] = s.ObjectSpecIndex[ref].Entity
	}
	return result
}

func (s *State) EntityCount() int {
	return len(s.ObjectSpecIndex)
}

func (s *State) Append(t *starlark.Thread, entities []k8s.K8sEntity, dupesOK bool) error {
	stackTrace := t.CallStack().String()
	for _, e := range entities {
		ref := e.ToObjectReference()
		old, exists := s.ObjectSpecIndex[ref]
		if exists && !dupesOK {
			humanRef := ""
			if ref.Namespace == "" {
				humanRef = fmt.Sprintf("%s %s", ref.Kind, ref.Name)
			} else {
				humanRef = fmt.Sprintf("%s %s (Namespace: %s)", ref.Kind, ref.Name, ref.Namespace)
			}
			return DuplicateYAMLDetectedError(humanRef, old.StackTrace)
		}

		if !exists {
			s.ObjectSpecRefs = append(s.ObjectSpecRefs, ref)
		}
		s.ObjectSpecIndex[ref] = ObjectSpec{
			Entity:     e,
			StackTrace: stackTrace,
		}
	}
	return nil
}

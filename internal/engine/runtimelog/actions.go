package runtimelog

import (
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/store"
)

type PodLogStreamCreateAction struct {
	PodLogStream *PodLogStream
}

func NewPodLogStreamCreateAction(s *PodLogStream) PodLogStreamCreateAction {
	return PodLogStreamCreateAction{PodLogStream: s.DeepCopy()}
}

var _ store.Summarizer = PodLogStreamCreateAction{}

func (PodLogStreamCreateAction) Action() {}

func (a PodLogStreamCreateAction) Summarize(s *store.ChangeSummary) {
	s.PodLogStreams.Add(types.NamespacedName{Name: a.PodLogStream.Name})
}

type PodLogStreamDeleteAction struct {
	Name string
}

func (PodLogStreamDeleteAction) Action() {}

func (a PodLogStreamDeleteAction) Summarize(s *store.ChangeSummary) {
	s.PodLogStreams.Add(types.NamespacedName{Name: a.Name})
}

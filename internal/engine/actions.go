package engine

import "k8s.io/api/core/v1"

type ErrorAction struct {
	Error error
}

func (ErrorAction) Action() {}

func NewErrorAction(err error) ErrorAction {
	return ErrorAction{Error: err}
}

type PodChangeAction struct {
	Pod *v1.Pod
}

func (PodChangeAction) Action() {}

func NewPodChangeAction(pod *v1.Pod) PodChangeAction {
	return PodChangeAction{Pod: pod}
}

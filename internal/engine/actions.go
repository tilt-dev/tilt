package engine

import (
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"k8s.io/api/core/v1"
)

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

type ServiceChangeAction struct {
	Service *v1.Service
}

func (ServiceChangeAction) Action() {}

func NewServiceChangeAction(service *v1.Service) ServiceChangeAction {
	return ServiceChangeAction{Service: service}
}

type BuildCompleteAction struct {
	Result store.BuildResult
	Error  error
}

func (BuildCompleteAction) Action() {}

func NewBuildCompleteAction(result store.BuildResult, err error) BuildCompleteAction {
	return BuildCompleteAction{
		Result: result,
		Error:  err,
	}
}

type InitAction struct {
	WatchMounts bool
	Manifests   []model.Manifest
}

func (InitAction) Action() {}

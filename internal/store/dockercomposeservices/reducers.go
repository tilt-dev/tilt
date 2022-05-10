package dockercomposeservices

import (
	"fmt"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

func SpanIDForDCService(mn model.ManifestName) logstore.SpanID {
	return logstore.SpanID(fmt.Sprintf("dc:%s", mn))
}

func HandleDockerComposeServiceUpsertAction(state *store.EngineState, action DockerComposeServiceUpsertAction) {
	obj := action.DockerComposeService
	n := obj.Name
	state.DockerComposeServices[n] = obj

	mn := model.ManifestName(obj.GetAnnotations()[v1alpha1.AnnotationManifest])
	mt, ok := state.ManifestTargets[mn]
	if !ok || !mt.Manifest.IsDC() {
		return
	}

	dcs, ok := mt.State.RuntimeState.(dockercompose.State)
	if !ok {
		dcs = dockercompose.State{}
	}

	dcs = dcs.WithSpanID(SpanIDForDCService(mn))

	cid := obj.Status.ContainerID
	if cid != "" {
		dcs = dcs.WithContainerID(container.ID(cid))
	}

	cState := obj.Status.ContainerState
	if cState != nil {
		dcs = dcs.WithContainerState(*cState)
		dcs = dcs.WithPorts(obj.Status.PortBindings)
	}

	mt.State.RuntimeState = dcs
}

func HandleDockerComposeServiceDeleteAction(state *store.EngineState, action DockerComposeServiceDeleteAction) {
	delete(state.DockerComposeServices, action.Name)
}

package hud

import (
	"os"
	"sort"
	"sync"

	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/hud/view"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/ospath"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

func StateToTerminalView(s store.EngineState, mu *sync.RWMutex) view.View {
	ret := view.View{}

	for _, ms := range s.TiltfileStates {
		ret.Resources = append(ret.Resources, tiltfileResourceView(ms))
	}

	for _, name := range s.ManifestDefinitionOrder {
		mt, ok := s.ManifestTargets[name]
		if !ok {
			continue
		}

		ms := mt.State
		if ms.DisableState == v1alpha1.DisableStateDisabled {
			// Don't show disabled resources in the terminal UI.
			continue
		}

		var absWatchDirs []string
		for i, p := range mt.Manifest.LocalPaths() {
			if i > 50 {
				// Bail out after 50 to avoid pathological performance issues.
				break
			}
			fi, err := os.Stat(p)

			// Treat this as a directory when there's an error.
			if err != nil || fi.IsDir() {
				absWatchDirs = append(absWatchDirs, p)
			}
		}

		var pendingBuildEdits []string
		for _, status := range ms.BuildStatuses {
			pendingBuildEdits = append(pendingBuildEdits, status.PendingFileChangesList()...)
		}

		pendingBuildEdits = ospath.FileListDisplayNames(absWatchDirs, pendingBuildEdits)

		buildHistory := append([]model.BuildRecord{}, ms.BuildHistory...)
		for i, build := range buildHistory {
			build.Edits = ospath.FileListDisplayNames(absWatchDirs, build.Edits)
			buildHistory[i] = build
		}

		currentBuild := ms.EarliestCurrentBuild()
		currentBuild.Edits = ospath.FileListDisplayNames(absWatchDirs, currentBuild.Edits)

		// Sort the strings to make the outputs deterministic.
		sort.Strings(pendingBuildEdits)

		endpoints := store.ManifestTargetEndpoints(mt)

		// NOTE(nick): Right now, the UX is designed to show the output exactly one
		// pod. A better UI might summarize the pods in other ways (e.g., show the
		// "most interesting" pod that's crash looping, or show logs from all pods
		// at once).
		_, pendingBuildSince := ms.HasPendingChanges()
		r := view.Resource{
			Name:               name,
			LastDeployTime:     ms.LastSuccessfulDeployTime,
			TriggerMode:        mt.Manifest.TriggerMode,
			BuildHistory:       buildHistory,
			PendingBuildEdits:  pendingBuildEdits,
			PendingBuildSince:  pendingBuildSince,
			PendingBuildReason: mt.NextBuildReason(),
			CurrentBuild:       currentBuild,
			Endpoints:          model.LinksToURLStrings(endpoints), // hud can't handle link names, just send URLs
			ResourceInfo:       resourceInfoView(mt),
		}

		ret.Resources = append(ret.Resources, r)
	}

	ret.LogReader = logstore.NewReader(mu, s.LogStore)
	ret.FatalError = s.FatalError

	return ret
}

const MainTiltfileManifestName = model.MainTiltfileManifestName

func tiltfileResourceView(ms *store.ManifestState) view.Resource {
	currentBuild := ms.EarliestCurrentBuild()
	tr := view.Resource{
		Name:         MainTiltfileManifestName,
		IsTiltfile:   true,
		CurrentBuild: currentBuild,
		BuildHistory: ms.BuildHistory,
		ResourceInfo: view.TiltfileResourceInfo{},
	}
	if !currentBuild.Empty() {
		tr.PendingBuildSince = currentBuild.StartTime
	} else {
		tr.LastDeployTime = ms.LastBuild().FinishTime
	}
	return tr
}

func resourceInfoView(mt *store.ManifestTarget) view.ResourceInfoView {
	runStatus := mt.RuntimeStatus()
	switch state := mt.State.RuntimeState.(type) {
	case dockercompose.State:
		return view.NewDCResourceInfo(
			state.ContainerState.Status, state.ContainerID, state.SpanID, state.ContainerState.StartedAt.Time, runStatus)
	case store.K8sRuntimeState:
		if mt.Manifest.PodReadinessMode() == model.PodReadinessIgnore {
			return view.YAMLResourceInfo{
				K8sDisplayNames: state.EntityDisplayNames(),
			}
		}
		pod := state.MostRecentPod()
		podID := k8s.PodID(pod.Name)
		return view.K8sResourceInfo{
			PodName:            pod.Name,
			PodCreationTime:    pod.CreatedAt.Time,
			PodUpdateStartTime: state.UpdateStartTime[podID],
			PodStatus:          pod.Status,
			PodRestarts:        int(state.VisiblePodContainerRestarts(podID)),
			SpanID:             k8sconv.SpanIDForPod(mt.Manifest.Name, podID),
			RunStatus:          runStatus,
			DisplayNames:       state.EntityDisplayNames(),
		}
	case store.LocalRuntimeState:
		return view.NewLocalResourceInfo(runStatus, state.PID, state.SpanID)
	default:
		// This is silly but it was the old behavior.
		return view.K8sResourceInfo{}
	}
}

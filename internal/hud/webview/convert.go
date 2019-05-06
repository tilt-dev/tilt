package webview

import (
	"os"
	"sort"

	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
	"github.com/windmilleng/tilt/internal/store"
)

func StateToWebView(s store.EngineState) View {
	ret := View{}

	ret.Resources = append(ret.Resources, tiltfileResourceView(s))

	for _, name := range s.ManifestDefinitionOrder {
		mt, ok := s.ManifestTargets[name]
		if !ok {
			continue
		}

		ms := mt.State

		var absWatchDirs []string
		var absWatchPaths []string
		for _, p := range mt.Manifest.LocalPaths() {
			fi, err := os.Stat(p)
			if err == nil && !fi.IsDir() {
				absWatchPaths = append(absWatchPaths, p)
			} else {
				absWatchDirs = append(absWatchDirs, p)
			}
		}
		absWatchPaths = append(absWatchPaths, s.TiltfilePath)
		relWatchDirs := ospath.TryAsCwdChildren(absWatchDirs)
		relWatchPaths := ospath.TryAsCwdChildren(absWatchPaths)

		var pendingBuildEdits []string
		for _, status := range ms.BuildStatuses {
			for f := range status.PendingFileChanges {
				pendingBuildEdits = append(pendingBuildEdits, f)
			}
		}

		pendingBuildEdits = ospath.FileListDisplayNames(absWatchDirs, pendingBuildEdits)

		buildHistory := append([]model.BuildRecord{}, ms.BuildHistory...)
		for i, build := range buildHistory {
			build.Edits = ospath.FileListDisplayNames(absWatchDirs, build.Edits)
			buildHistory[i] = build
		}

		currentBuild := ms.CurrentBuild
		currentBuild.Edits = ospath.FileListDisplayNames(absWatchDirs, ms.CurrentBuild.Edits)

		// Sort the strings to make the outputs deterministic.
		sort.Strings(pendingBuildEdits)

		endpoints := store.ManifestTargetEndpoints(mt)

		podID := ms.MostRecentPod().PodID

		// NOTE(nick): Right now, the UX is designed to show the output exactly one
		// pod. A better UI might summarize the pods in other ways (e.g., show the
		// "most interesting" pod that's crash looping, or show logs from all pods
		// at once).
		_, pendingBuildSince := ms.HasPendingChanges()
		r := Resource{
			Name:               name,
			DirectoriesWatched: relWatchDirs,
			PathsWatched:       relWatchPaths,
			LastDeployTime:     ms.LastSuccessfulDeployTime,
			BuildHistory:       buildHistory,
			PendingBuildEdits:  pendingBuildEdits,
			PendingBuildSince:  pendingBuildSince,
			PendingBuildReason: ms.NextBuildReason(),
			CurrentBuild:       currentBuild,
			Endpoints:          endpoints,
			PodID:              podID,
			ResourceInfo:       resourceInfoView(mt),
			ShowBuildStatus:    len(mt.Manifest.ImageTargets) > 0 || mt.Manifest.IsDC(),
			CombinedLog:        ms.CombinedLog,
		}

		r.RuntimeStatus = runtimeStatus(r.ResourceInfo)

		ret.Resources = append(ret.Resources, r)
	}

	// if s.GlobalYAML.K8sTarget().YAML != "" {
	// 	absWatches := append([]string{}, s.ConfigFiles...)
	// 	relWatches := ospath.TryAsCwdChildren(absWatches)
	//
	// 	r := Resource{
	// 		Name:               s.GlobalYAML.ManifestName(),
	// 		DirectoriesWatched: relWatches,
	// 		CurrentBuild:       s.GlobalYAMLState.ActiveBuild(),
	// 		BuildHistory: []model.BuildRecord{
	// 			s.GlobalYAMLState.LastBuild(),
	// 		},
	// 		LastDeployTime: s.GlobalYAMLState.LastSuccessfulApplyTime,
	// 		ResourceInfo: YAMLResourceInfo{
	// 			K8sResources: s.GlobalYAML.K8sTarget().ResourceNames,
	// 		},
	// 	}
	//
	// 	r.RuntimeStatus = runtimeStatus(r.ResourceInfo)
	//
	// 	ret.Resources = append(ret.Resources, r)
	// }

	ret.Log = s.Log
	ret.SailEnabled = s.SailEnabled
	ret.SailURL = s.SailURL

	return ret
}

func tiltfileResourceView(s store.EngineState) Resource {
	ltfb := s.LastTiltfileBuild
	if !s.CurrentTiltfileBuild.Empty() {
		ltfb.Log = s.CurrentTiltfileBuild.Log
	}
	tr := Resource{
		Name:         view.TiltfileResourceName,
		IsTiltfile:   true,
		CurrentBuild: s.CurrentTiltfileBuild,
		BuildHistory: []model.BuildRecord{
			ltfb,
		},
		CombinedLog:   s.TiltfileCombinedLog,
		RuntimeStatus: RuntimeStatusOK,
	}
	if !s.CurrentTiltfileBuild.Empty() {
		tr.PendingBuildSince = s.CurrentTiltfileBuild.StartTime
	} else {
		tr.LastDeployTime = s.LastTiltfileBuild.FinishTime
	}
	return tr
}

func resourceInfoView(mt *store.ManifestTarget) ResourceInfoView {
	if mt.Manifest.Name == model.UnresourcedYAMLManifestName {
		return YAMLResourceInfo{
			K8sResources: mt.Manifest.K8sTarget().ResourceNames,
		}
	}
	if dcState, ok := mt.State.ResourceState.(dockercompose.State); ok {
		return NewDCResourceInfo(mt.Manifest.DockerComposeTarget().ConfigPath, dcState.Status, dcState.ContainerID, dcState.Log(), dcState.StartTime)
	} else {
		pod := mt.State.MostRecentPod()
		return K8SResourceInfo{
			PodName:            pod.PodID.String(),
			PodCreationTime:    pod.StartedAt,
			PodUpdateStartTime: pod.UpdateStartTime,
			PodStatus:          pod.Status,
			PodRestarts:        pod.ContainerRestarts - pod.OldRestarts,
			PodLog:             pod.Log(),
			YAML:               mt.Manifest.K8sTarget().YAML,
		}
	}
}

func runtimeStatus(res ResourceInfoView) RuntimeStatus {
	// if we have no images to build, we have no runtime status monitoring.
	_, isYAML := res.(YAMLResourceInfo)
	if isYAML {
		return RuntimeStatusOK
	}

	result, ok := runtimeStatusMap[res.Status()]
	if !ok {
		return RuntimeStatusError
	}
	return result
}

var runtimeStatusMap = map[string]RuntimeStatus{
	"Running":                          RuntimeStatusOK,
	"ContainerCreating":                RuntimeStatusPending,
	"Pending":                          RuntimeStatusPending,
	"PodInitializing":                  RuntimeStatusPending,
	"Error":                            RuntimeStatusError,
	"CrashLoopBackOff":                 RuntimeStatusError,
	"ErrImagePull":                     RuntimeStatusError,
	"ImagePullBackOff":                 RuntimeStatusError,
	string(dockercompose.StatusInProg): RuntimeStatusPending,
	string(dockercompose.StatusUp):     RuntimeStatusOK,
	string(dockercompose.StatusDown):   RuntimeStatusError,
	"Completed":                        RuntimeStatusOK,

	// If the runtime status hasn't shown up yet, we assume it's pending.
	"": RuntimeStatusPending,
}

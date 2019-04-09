package server

import (
	"os"
	"sort"

	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/hud/webview"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
	"github.com/windmilleng/tilt/internal/store"
)

func StateToWebView(s store.EngineState) webview.View {
	ret := webview.View{}

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

		// NOTE(nick): Right now, the UX is designed to show the output exactly one
		// pod. A better UI might summarize the pods in other ways (e.g., show the
		// "most interesting" pod that's crash looping, or show logs from all pods
		// at once).
		_, pendingBuildSince := ms.HasPendingChanges()
		r := webview.Resource{
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
			ResourceInfo:       resourceInfoView(mt),
			ShowBuildStatus:    len(mt.Manifest.ImageTargets) > 0 || mt.Manifest.IsDC(),
			CombinedLog:        ms.CombinedLog,
		}

		r.RuntimeStatus = runtimeStatus(r.ResourceInfo)

		ret.Resources = append(ret.Resources, r)
	}

	if s.GlobalYAML.K8sTarget().YAML != "" {
		absWatches := append([]string{}, s.ConfigFiles...)
		relWatches := ospath.TryAsCwdChildren(absWatches)

		r := webview.Resource{
			Name:               s.GlobalYAML.ManifestName(),
			DirectoriesWatched: relWatches,
			CurrentBuild:       s.GlobalYAMLState.ActiveBuild(),
			BuildHistory: []model.BuildRecord{
				s.GlobalYAMLState.LastBuild(),
			},
			LastDeployTime: s.GlobalYAMLState.LastSuccessfulApplyTime,
			ResourceInfo: webview.YAMLResourceInfo{
				K8sResources: s.GlobalYAML.K8sTarget().ResourceNames,
			},
		}

		r.RuntimeStatus = runtimeStatus(r.ResourceInfo)

		ret.Resources = append(ret.Resources, r)
	}

	ltfb := s.LastTiltfileBuild
	if !s.CurrentTiltfileBuild.Empty() {
		ltfb.Log = s.CurrentTiltfileBuild.Log
	}
	tr := webview.Resource{
		Name:         view.TiltfileResourceName,
		IsTiltfile:   true,
		CurrentBuild: s.CurrentTiltfileBuild,
		BuildHistory: []model.BuildRecord{
			ltfb,
		},
		CombinedLog:   s.TiltfileCombinedLog,
		RuntimeStatus: webview.RuntimeStatusOK,
	}
	if !s.CurrentTiltfileBuild.Empty() {
		tr.PendingBuildSince = s.CurrentTiltfileBuild.StartTime
	} else {
		tr.LastDeployTime = s.LastTiltfileBuild.FinishTime
	}
	ret.Resources = append(ret.Resources, tr)

	ret.Log = s.Log

	return ret
}

func resourceInfoView(mt *store.ManifestTarget) webview.ResourceInfoView {
	if dcState, ok := mt.State.ResourceState.(dockercompose.State); ok {
		return webview.NewDCResourceInfo(mt.Manifest.DockerComposeTarget().ConfigPath, dcState.Status, dcState.ContainerID, dcState.Log(), dcState.StartTime)
	} else {
		pod := mt.State.MostRecentPod()
		return webview.K8SResourceInfo{
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

func runtimeStatus(res webview.ResourceInfoView) webview.RuntimeStatus {
	// if we have no images to build, we have no runtime status monitoring.
	_, isYAML := res.(webview.YAMLResourceInfo)
	if isYAML {
		return webview.RuntimeStatusOK
	}

	result, ok := runtimeStatusMap[res.Status()]
	if !ok {
		return webview.RuntimeStatusError
	}
	return result
}

var runtimeStatusMap = map[string]webview.RuntimeStatus{
	"Running":                          webview.RuntimeStatusOK,
	"ContainerCreating":                webview.RuntimeStatusPending,
	"Pending":                          webview.RuntimeStatusPending,
	"PodInitializing":                  webview.RuntimeStatusPending,
	"Error":                            webview.RuntimeStatusError,
	"CrashLoopBackOff":                 webview.RuntimeStatusError,
	"ErrImagePull":                     webview.RuntimeStatusError,
	"ImagePullBackOff":                 webview.RuntimeStatusError,
	string(dockercompose.StatusInProg): webview.RuntimeStatusPending,
	string(dockercompose.StatusUp):     webview.RuntimeStatusOK,
	string(dockercompose.StatusDown):   webview.RuntimeStatusError,
	"Completed":                        webview.RuntimeStatusOK,

	// If the runtime status hasn't shown up yet, we assume it's pending.
	"": webview.RuntimeStatusPending,
}

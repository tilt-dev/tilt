package webview

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

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
		hasPendingChanges, pendingBuildSince := ms.HasPendingChanges()
		r := Resource{
			Name:               name,
			DirectoriesWatched: relWatchDirs,
			PathsWatched:       relWatchPaths,
			LastDeployTime:     ms.LastSuccessfulDeployTime,
			BuildHistory:       ToWebViewBuildRecords(buildHistory),
			PendingBuildEdits:  pendingBuildEdits,
			PendingBuildSince:  pendingBuildSince,
			PendingBuildReason: ms.NextBuildReason(),
			CurrentBuild:       ToWebViewBuildRecord(currentBuild),
			Endpoints:          endpoints,
			PodID:              podID,
			ResourceInfo:       resourceInfoView(mt),
			ShowBuildStatus:    len(mt.Manifest.ImageTargets) > 0 || mt.Manifest.IsDC(),
			CombinedLog:        ms.CombinedLog,
			CrashLog:           ms.CrashLog,
			TriggerMode:        mt.Manifest.TriggerMode,
			HasPendingChanges:  hasPendingChanges,
		}

		r.RuntimeStatus = runtimeStatus(r.ResourceInfo)

		ret.Resources = append(ret.Resources, r)
	}

	ret.Log = s.Log
	ret.SailEnabled = s.SailEnabled
	ret.SailURL = s.SailURL
	ret.NeedsAnalyticsNudge = NeedsNudge(s)
	ret.RunningTiltBuild = s.TiltBuildInfo
	ret.LatestTiltBuild = s.LatestTiltBuild
	ret.FeatureFlags = s.Features

	return ret
}

func tiltfileResourceView(s store.EngineState) Resource {
	ltfb := s.LastTiltfileBuild
	if !s.CurrentTiltfileBuild.Empty() {
		ltfb.Log = s.CurrentTiltfileBuild.Log
	}

	ltfb.Edits = ospath.FileListDisplayNames([]string{filepath.Dir(s.TiltfilePath)}, ltfb.Edits)

	tr := Resource{
		Name:         view.TiltfileResourceName,
		IsTiltfile:   true,
		CurrentBuild: ToWebViewBuildRecord(s.CurrentTiltfileBuild),
		BuildHistory: []BuildRecord{
			ToWebViewBuildRecord(ltfb),
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
	if mt.Manifest.IsUnresourcedYAMLManifest() {
		return YAMLResourceInfo{
			K8sResources: mt.Manifest.K8sTarget().DisplayNames,
		}
	}
	if dcState, ok := mt.State.ResourceState.(dockercompose.State); ok {
		return NewDCResourceInfo(mt.Manifest.DockerComposeTarget().ConfigPaths, dcState.Status, dcState.ContainerID, dcState.Log(), dcState.StartTime)
	} else {
		pod := mt.State.MostRecentPod()
		return K8sResourceInfo{
			PodName:            pod.PodID.String(),
			PodCreationTime:    pod.StartedAt,
			PodUpdateStartTime: pod.UpdateStartTime,
			PodStatus:          pod.Status,
			PodStatusMessage:   strings.Join(pod.StatusMessages, "\n"),
			PodRestarts:        pod.AllContainerRestarts() - pod.OldRestarts,
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
	"RunContainerError":                RuntimeStatusError,
	"StartError":                       RuntimeStatusError,
	string(dockercompose.StatusInProg): RuntimeStatusPending,
	string(dockercompose.StatusUp):     RuntimeStatusOK,
	string(dockercompose.StatusDown):   RuntimeStatusError,
	"Completed":                        RuntimeStatusOK,

	// If the runtime status hasn't shown up yet, we assume it's pending.
	"": RuntimeStatusPending,
}

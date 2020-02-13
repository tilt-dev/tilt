package webview

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/windmilleng/tilt/internal/cloud/cloudurl"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/feature"
	"github.com/windmilleng/tilt/internal/ospath"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"
	"github.com/windmilleng/tilt/pkg/model/logstore"

	proto_webview "github.com/windmilleng/tilt/pkg/webview"
)

func StateToProtoView(s store.EngineState, logCheckpoint logstore.Checkpoint) (*proto_webview.View, error) {
	ret := &proto_webview.View{}

	rpv, err := tiltfileResourceProtoView(s)
	if err != nil {
		return nil, err
	}
	ret.Resources = append(ret.Resources, rpv)

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

		var facets []model.Facet
		if s.Features[feature.Facets] {
			facets = mt.Facets(s.Secrets)
		}

		bh, err := ToProtoBuildRecords(buildHistory, s.LogStore)
		if err != nil {
			return nil, err
		}
		lastDeploy, err := timeToProto(ms.LastSuccessfulDeployTime)
		if err != nil {
			return nil, err
		}
		cb, err := ToProtoBuildRecord(currentBuild, s.LogStore)
		if err != nil {
			return nil, err
		}

		targetTypes, err := TargetsToProtoBuildTypes(mt.Manifest.TargetSpecs())
		if err != nil {
			return nil, err
		}

		// NOTE(nick): Right now, the UX is designed to show the output exactly one
		// pod. A better UI might summarize the pods in other ways (e.g., show the
		// "most interesting" pod that's crash looping, or show logs from all pods
		// at once).
		hasPendingChanges, pendingBuildSince := ms.HasPendingChanges()
		pbs, err := timeToProto(pendingBuildSince)
		if err != nil {
			return nil, err
		}

		r := &proto_webview.Resource{
			Name:               name.String(),
			DirectoriesWatched: relWatchDirs,
			PathsWatched:       relWatchPaths,
			LastDeployTime:     lastDeploy,
			BuildHistory:       bh,
			PendingBuildEdits:  pendingBuildEdits,
			PendingBuildSince:  pbs,
			PendingBuildReason: int32(mt.NextBuildReason()),
			CurrentBuild:       cb,
			Endpoints:          endpoints,
			PodID:              podID.String(),
			TargetTypes:        targetTypes,
			ShowBuildStatus:    len(mt.Manifest.ImageTargets) > 0 || mt.Manifest.IsDC(),
			CrashLog:           ms.CrashLog.String(),
			TriggerMode:        int32(mt.Manifest.TriggerMode),
			HasPendingChanges:  hasPendingChanges,
			Facets:             model.FacetsToProto(facets),
			Queued:             s.ManifestInTriggerQueue(name),
		}

		err = protoPopulateResourceInfoView(mt, r)
		if err != nil {
			return nil, err
		}

		ret.Resources = append(ret.Resources, r)
	}

	logList, err := s.LogStore.ToLogList(logCheckpoint)
	if err != nil {
		return nil, err
	}

	ret.LogList = logList
	ret.NeedsAnalyticsNudge = NeedsNudge(s)
	ret.RunningTiltBuild = &proto_webview.TiltBuild{
		Version:   s.TiltBuildInfo.Version,
		CommitSHA: s.TiltBuildInfo.CommitSHA,
		Dev:       s.TiltBuildInfo.Dev,
		Date:      s.TiltBuildInfo.Date,
	}
	ret.LatestTiltBuild = &proto_webview.TiltBuild{
		Version:   s.LatestTiltBuild.Version,
		CommitSHA: s.LatestTiltBuild.CommitSHA,
		Dev:       s.LatestTiltBuild.Dev,
		Date:      s.LatestTiltBuild.Date,
	}
	ret.FeatureFlags = make(map[string]bool)
	for k, v := range s.Features {
		ret.FeatureFlags[k] = v
	}
	ret.TiltCloudUsername = s.TiltCloudUsername
	ret.TiltCloudSchemeHost = cloudurl.URL(s.CloudAddress).String()
	ret.TiltCloudTeamID = s.TeamName
	if s.FatalError != nil {
		ret.FatalError = s.FatalError.Error()
	}

	ret.VersionSettings = &proto_webview.VersionSettings{
		CheckUpdates: s.VersionSettings.CheckUpdates,
	}

	start, err := timeToProto(s.TiltStartTime)
	if err != nil {
		return nil, err
	}
	ret.TiltStartTime = start

	return ret, nil
}

func tiltfileResourceProtoView(s store.EngineState) (*proto_webview.Resource, error) {
	ltfb := s.TiltfileState.LastBuild()
	ctfb := s.TiltfileState.CurrentBuild

	ltfb.Edits = ospath.FileListDisplayNames([]string{filepath.Dir(s.TiltfilePath)}, ltfb.Edits)

	pctfb, err := ToProtoBuildRecord(ctfb, s.LogStore)
	if err != nil {
		return nil, err
	}
	pltfb, err := ToProtoBuildRecord(ltfb, s.LogStore)
	if err != nil {
		return nil, err
	}
	tr := &proto_webview.Resource{
		Name:         store.TiltfileManifestName.String(),
		IsTiltfile:   true,
		CurrentBuild: pctfb,
		BuildHistory: []*proto_webview.BuildRecord{
			pltfb,
		},
		RuntimeStatus: string(model.RuntimeStatusOK),
	}
	start, err := timeToProto(ctfb.StartTime)
	if err != nil {
		return nil, err
	}
	finish, err := timeToProto(ltfb.FinishTime)
	if err != nil {
		return nil, err
	}
	if !ctfb.Empty() {
		tr.PendingBuildSince = start
	} else {
		tr.LastDeployTime = finish
	}
	return tr, nil
}

func protoPopulateResourceInfoView(mt *store.ManifestTarget, r *proto_webview.Resource) error {
	r.RuntimeStatus = string(model.RuntimeStatusNotApplicable)

	if mt.Manifest.IsUnresourcedYAMLManifest() {
		r.YamlResourceInfo = &proto_webview.YAMLResourceInfo{
			K8SResources: mt.Manifest.K8sTarget().DisplayNames,
		}
		return nil
	}

	if mt.Manifest.IsDC() {
		dc := mt.Manifest.DockerComposeTarget()
		dcState := mt.State.DCRuntimeState()
		info, err := NewProtoDCResourceInfo(dc.ConfigPaths, dcState.Status, dcState.ContainerID, dcState.StartTime)
		if err != nil {
			return err
		}
		r.DcResourceInfo = info

		runtimeStatus, ok := runtimeStatusMap[string(dcState.Status)]
		if !ok {
			r.RuntimeStatus = string(model.RuntimeStatusError)
		}
		r.RuntimeStatus = string(runtimeStatus)
		return nil
	}
	if mt.Manifest.IsLocal() {
		lState := mt.State.LocalRuntimeState()
		r.LocalResourceInfo = &proto_webview.LocalResourceInfo{Pid: int64(lState.PID)}
		r.RuntimeStatus = string(lState.Status)
		return nil
	}
	if mt.Manifest.IsK8s() {
		kState := mt.State.K8sRuntimeState()
		pod := kState.MostRecentPod()
		r.K8SResourceInfo = &proto_webview.K8SResourceInfo{
			PodName:            pod.PodID.String(),
			PodCreationTime:    pod.StartedAt.String(),
			PodUpdateStartTime: pod.UpdateStartTime.String(),
			PodStatus:          pod.Status,
			PodStatusMessage:   strings.Join(pod.StatusMessages, "\n"),
			AllContainersReady: pod.AllContainersReady(),
			PodRestarts:        int32(pod.VisibleContainerRestarts()),
		}

		status := pod.Status
		if status == "Running" && !pod.AllContainersReady() {
			status = "Pending"
		}

		runtimeStatus, ok := runtimeStatusMap[status]
		if !ok {
			r.RuntimeStatus = string(model.RuntimeStatusError)
		}
		r.RuntimeStatus = string(runtimeStatus)
		return nil
	}

	panic("Unrecognized manifest type (not one of: k8s, DC, local)")
}

var runtimeStatusMap = map[string]model.RuntimeStatus{
	"Running":                          model.RuntimeStatusOK,
	"ContainerCreating":                model.RuntimeStatusPending,
	"Pending":                          model.RuntimeStatusPending,
	"PodInitializing":                  model.RuntimeStatusPending,
	"Error":                            model.RuntimeStatusError,
	"CrashLoopBackOff":                 model.RuntimeStatusError,
	"ErrImagePull":                     model.RuntimeStatusError,
	"ImagePullBackOff":                 model.RuntimeStatusError,
	"RunContainerError":                model.RuntimeStatusError,
	"StartError":                       model.RuntimeStatusError,
	string(dockercompose.StatusInProg): model.RuntimeStatusPending,
	string(dockercompose.StatusUp):     model.RuntimeStatusOK,
	string(dockercompose.StatusDown):   model.RuntimeStatusError,
	"Completed":                        model.RuntimeStatusOK,

	// If the runtime status hasn't shown up yet, we assume it's pending.
	"": model.RuntimeStatusPending,
}

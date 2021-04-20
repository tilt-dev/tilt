package webview

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tilt-dev/tilt/internal/cloud/cloudurl"
	"github.com/tilt-dev/tilt/internal/ospath"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"

	proto_webview "github.com/tilt-dev/tilt/pkg/webview"
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

		// Skip manifests that don't come from the tiltfile.
		if mt.Manifest.Source != model.ManifestSourceTiltfile {
			continue
		}

		ms := mt.State

		var absWatchDirs []string
		for i, p := range mt.Manifest.LocalPaths() {
			if i > 50 {
				// to avoid pathological perf cases, stop after 50
				break
			}
			fi, err := os.Stat(p)

			// Treat this as a directory if there's an error
			if err != nil || fi.IsDir() {
				absWatchDirs = append(absWatchDirs, p)
			}
		}

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

		podID := ms.MostRecentPod().Name

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

		specs, err := TargetSpecsToProto(mt.Manifest.TargetSpecs())
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
			LastDeployTime:     lastDeploy,
			BuildHistory:       bh,
			PendingBuildEdits:  pendingBuildEdits,
			PendingBuildSince:  pbs,
			PendingBuildReason: int32(mt.NextBuildReason()),
			CurrentBuild:       cb,
			EndpointLinks:      ToProtoLinks(endpoints),
			PodID:              podID,
			Specs:              specs,
			ShowBuildStatus:    len(mt.Manifest.ImageTargets) > 0 || mt.Manifest.IsDC(),
			TriggerMode:        int32(mt.Manifest.TriggerMode),
			HasPendingChanges:  hasPendingChanges,
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
	ret.SuggestedTiltVersion = s.SuggestedTiltVersion
	ret.FeatureFlags = make(map[string]bool)
	for k, v := range s.Features {
		ret.FeatureFlags[k] = v
	}
	ret.TiltCloudUsername = s.CloudStatus.Username
	ret.TiltCloudTeamName = s.CloudStatus.TeamName
	ret.TiltCloudSchemeHost = cloudurl.URL(s.CloudAddress).String()
	ret.TiltCloudTeamID = s.TeamID
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

	ret.TiltfileKey = s.TiltfilePath
	ret.MetricsServing = toMetricsServingProto(s.MetricsServing)

	return ret, nil
}

func toMetricsServingProto(s store.MetricsServing) *proto_webview.MetricsServing {
	return &proto_webview.MetricsServing{
		Mode:        string(s.Mode),
		GrafanaHost: s.GrafanaHost,
	}
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
		RuntimeStatus: string(model.RuntimeStatusNotApplicable),
		UpdateStatus:  string(s.TiltfileState.UpdateStatus(model.TriggerModeAuto)),
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
	r.UpdateStatus = string(mt.UpdateStatus())
	r.RuntimeStatus = string(model.RuntimeStatusNotApplicable)

	if mt.Manifest.PodReadinessMode() == model.PodReadinessIgnore {
		r.YamlResourceInfo = &proto_webview.YAMLResourceInfo{
			K8SResources: mt.Manifest.K8sTarget().DisplayNames,
		}
		return nil
	}

	if mt.Manifest.IsDC() {
		dc := mt.Manifest.DockerComposeTarget()
		dcState := mt.State.DCRuntimeState()
		info, err := NewProtoDCResourceInfo(dc.ConfigPaths, dcState.ContainerState.Status, dcState.ContainerID, dcState.StartTime)
		if err != nil {
			return err
		}
		r.DcResourceInfo = info
		r.RuntimeStatus = string(dcState.RuntimeStatus())
		return nil
	}
	if mt.Manifest.IsLocal() {
		lState := mt.State.LocalRuntimeState()
		r.LocalResourceInfo = &proto_webview.LocalResourceInfo{Pid: int64(lState.PID), IsTest: mt.Manifest.LocalTarget().IsTest}
		r.RuntimeStatus = string(lState.RuntimeStatus())
		return nil
	}
	if mt.Manifest.IsK8s() {
		kState := mt.State.K8sRuntimeState()
		pod := kState.MostRecentPod()
		r.K8SResourceInfo = &proto_webview.K8SResourceInfo{
			PodName:            pod.Name,
			PodCreationTime:    pod.CreatedAt.String(),
			PodUpdateStartTime: pod.UpdateStartedAt.String(),
			PodStatus:          pod.Status,
			PodStatusMessage:   strings.Join(pod.Errors, "\n"),
			AllContainersReady: store.AllPodContainersReady(pod),
			PodRestarts:        int32(store.VisiblePodContainerRestarts(pod)),
			DisplayNames:       mt.Manifest.K8sTarget().DisplayNames,
		}

		r.RuntimeStatus = string(kState.RuntimeStatus())
		return nil
	}

	panic("Unrecognized manifest type (not one of: k8s, DC, local)")
}

func LogSegmentToEvent(seg *proto_webview.LogSegment, spans map[string]*proto_webview.LogSpan) store.LogAction {
	span, ok := spans[seg.SpanId]
	if !ok {
		// nonexistent span, ignore
		return store.LogAction{}
	}

	// TODO(maia): actually get level (just spoofing for now)
	spoofedLevel := logger.InfoLvl
	return store.NewLogAction(model.ManifestName(span.ManifestName), logstore.SpanID(seg.SpanId), spoofedLevel, seg.Fields, []byte(seg.Text))
}

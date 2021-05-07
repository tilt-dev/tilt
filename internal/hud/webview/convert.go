package webview

import (
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/cloud/cloudurl"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"

	proto_webview "github.com/tilt-dev/tilt/pkg/webview"
)

func StateToProtoView(s store.EngineState, logCheckpoint logstore.Checkpoint) (*proto_webview.View, error) {
	ret := &proto_webview.View{}

	rpv := tiltfileResourceProtoView(s)
	ret.UiResources = append(ret.UiResources, rpv)

	for _, name := range s.ManifestDefinitionOrder {
		mt, ok := s.ManifestTargets[name]
		if !ok {
			continue
		}

		// Skip manifests that don't come from the tiltfile.
		if mt.Manifest.Source != model.ManifestSourceTiltfile {
			continue
		}

		r, err := toUIResource(mt, s)
		if err != nil {
			return nil, err
		}

		ret.UiResources = append(ret.UiResources, r)
	}

	logList, err := s.LogStore.ToLogList(logCheckpoint)
	if err != nil {
		return nil, err
	}

	ret.LogList = logList
	ret.UiSession = toUISession(s)

	// We grandfather in TiltStartTime from the old protocol,
	// because it tells the UI to reload.
	start, err := timeToProto(s.TiltStartTime)
	if err != nil {
		return nil, err
	}
	ret.TiltStartTime = start

	return ret, nil
}

// Converts EngineState into the public data model representation, a UISession.
func toUISession(s store.EngineState) *v1alpha1.UISession {
	ret := &v1alpha1.UISession{
		ObjectMeta: metav1.ObjectMeta{
			// We call the main session the Tiltfile session, for compatibility
			// with the other Session API.
			Name: "Tiltfile",
		},
		Status: v1alpha1.UISessionStatus{},
	}

	status := &(ret.Status)
	status.NeedsAnalyticsNudge = NeedsNudge(s)
	status.RunningTiltBuild = v1alpha1.TiltBuild{
		Version:   s.TiltBuildInfo.Version,
		CommitSHA: s.TiltBuildInfo.CommitSHA,
		Dev:       s.TiltBuildInfo.Dev,
		Date:      s.TiltBuildInfo.Date,
	}
	status.SuggestedTiltVersion = s.SuggestedTiltVersion
	status.FeatureFlags = []v1alpha1.UIFeatureFlag{}
	for k, v := range s.Features {
		status.FeatureFlags = append(status.FeatureFlags, v1alpha1.UIFeatureFlag{
			Name:  k,
			Value: v,
		})
	}
	sort.Slice(status.FeatureFlags, func(i, j int) bool {
		return status.FeatureFlags[i].Name < status.FeatureFlags[j].Name
	})
	status.TiltCloudUsername = s.CloudStatus.Username
	status.TiltCloudTeamName = s.CloudStatus.TeamName
	status.TiltCloudSchemeHost = cloudurl.URL(s.CloudAddress).String()
	status.TiltCloudTeamID = s.TeamID
	if s.FatalError != nil {
		status.FatalError = s.FatalError.Error()
	}

	status.VersionSettings = v1alpha1.VersionSettings{
		CheckUpdates: s.VersionSettings.CheckUpdates,
	}

	status.TiltStartTime = metav1.NewTime(s.TiltStartTime)

	status.TiltfileKey = s.TiltfilePath

	return ret
}

// Converts a ManifestTarget into the public data model representation,
// a UIResource.
func toUIResource(mt *store.ManifestTarget, s store.EngineState) (*v1alpha1.UIResource, error) {
	ms := mt.State
	endpoints := store.ManifestTargetEndpoints(mt)

	bh := ToBuildsTerminated(ms.BuildHistory, s.LogStore)
	lastDeploy := metav1.NewMicroTime(ms.LastSuccessfulDeployTime)
	cb := ToBuildRunning(ms.CurrentBuild)

	specs, err := ToAPITargetSpecs(mt.Manifest.TargetSpecs())
	if err != nil {
		return nil, err
	}

	// NOTE(nick): Right now, the UX is designed to show the output exactly one
	// pod. A better UI might summarize the pods in other ways (e.g., show the
	// "most interesting" pod that's crash looping, or show logs from all pods
	// at once).
	hasPendingChanges, pendingBuildSince := ms.HasPendingChanges()

	name := mt.Manifest.Name
	r := &v1alpha1.UIResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: name.String(),
		},
		Status: v1alpha1.UIResourceStatus{
			LastDeployTime:    lastDeploy,
			BuildHistory:      bh,
			PendingBuildSince: metav1.NewMicroTime(pendingBuildSince),
			CurrentBuild:      cb,
			EndpointLinks:     ToAPILinks(endpoints),
			Specs:             specs,
			TriggerMode:       int32(mt.Manifest.TriggerMode),
			HasPendingChanges: hasPendingChanges,
			Queued:            s.ManifestInTriggerQueue(name),
		},
	}

	err = populateResourceInfoView(mt, r)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func tiltfileResourceProtoView(s store.EngineState) *v1alpha1.UIResource {
	ltfb := s.TiltfileState.LastBuild()
	ctfb := s.TiltfileState.CurrentBuild

	pctfb := ToBuildRunning(ctfb)
	history := []v1alpha1.UIBuildTerminated{}
	if !ltfb.Empty() {
		history = append(history, ToBuildTerminated(ltfb, s.LogStore))
	}
	tr := &v1alpha1.UIResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: store.TiltfileManifestName.String(),
		},
		Status: v1alpha1.UIResourceStatus{
			CurrentBuild:  pctfb,
			BuildHistory:  history,
			RuntimeStatus: v1alpha1.RuntimeStatusNotApplicable,
			UpdateStatus:  s.TiltfileState.UpdateStatus(model.TriggerModeAuto),
		},
	}
	start := metav1.NewMicroTime(ctfb.StartTime)
	finish := metav1.NewMicroTime(ltfb.FinishTime)
	if !ctfb.Empty() {
		tr.Status.PendingBuildSince = start
	} else {
		tr.Status.LastDeployTime = finish
	}
	return tr
}

func populateResourceInfoView(mt *store.ManifestTarget, r *v1alpha1.UIResource) error {
	r.Status.UpdateStatus = mt.UpdateStatus()
	r.Status.RuntimeStatus = v1alpha1.RuntimeStatusNotApplicable

	if mt.Manifest.PodReadinessMode() == model.PodReadinessIgnore {
		return nil
	}

	if mt.Manifest.IsDC() {
		dcState := mt.State.DCRuntimeState()
		r.Status.RuntimeStatus = v1alpha1.RuntimeStatus(dcState.RuntimeStatus())
		return nil
	}
	if mt.Manifest.IsLocal() {
		lState := mt.State.LocalRuntimeState()
		r.Status.LocalResourceInfo = &v1alpha1.UIResourceLocal{PID: int64(lState.PID), IsTest: mt.Manifest.LocalTarget().IsTest}
		r.Status.RuntimeStatus = v1alpha1.RuntimeStatus(lState.RuntimeStatus())
		return nil
	}
	if mt.Manifest.IsK8s() {
		kState := mt.State.K8sRuntimeState()
		pod := kState.MostRecentPod()
		r.Status.K8sResourceInfo = &v1alpha1.UIResourceKubernetes{
			PodName:            pod.Name,
			PodCreationTime:    pod.CreatedAt,
			PodUpdateStartTime: pod.UpdateStartedAt,
			PodStatus:          pod.Status,
			PodStatusMessage:   strings.Join(pod.Errors, "\n"),
			AllContainersReady: store.AllPodContainersReady(pod),
			PodRestarts:        int32(store.VisiblePodContainerRestarts(pod)),
			DisplayNames:       mt.Manifest.K8sTarget().DisplayNames,
		}

		r.Status.RuntimeStatus = v1alpha1.RuntimeStatus(kState.RuntimeStatus())
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

package webview

import (
	"context"
	"sort"
	"strings"

	"github.com/golang/protobuf/ptypes"

	"github.com/tilt-dev/tilt/internal/k8s"

	"github.com/tilt-dev/tilt/pkg/apis"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/cloud/cloudurl"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"

	proto_webview "github.com/tilt-dev/tilt/pkg/webview"
)

// We call the main session the Tiltfile session, for compatibility
// with the other Session API.
const UISessionName = "Tiltfile"

// Create the complete snapshot of the webview.
func CompleteView(ctx context.Context, client ctrlclient.Client, st store.RStore) (*proto_webview.View, error) {
	ret := &proto_webview.View{}
	session := &v1alpha1.UISession{}
	err := client.Get(ctx, types.NamespacedName{Name: UISessionName}, session)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}

	if err == nil {
		ret.UiSession = session
	}

	resourceList := &v1alpha1.UIResourceList{}
	err = client.List(ctx, resourceList)
	if err != nil {
		return nil, err
	}

	for _, item := range resourceList.Items {
		item := item
		ret.UiResources = append(ret.UiResources, &item)
	}

	buttonList := &v1alpha1.UIButtonList{}
	err = client.List(ctx, buttonList)
	if err != nil {
		return nil, err
	}

	for _, item := range buttonList.Items {
		item := item
		ret.UiButtons = append(ret.UiButtons, &item)
	}

	s := st.RLockState()
	defer st.RUnlockState()
	logList, err := s.LogStore.ToLogList(0)
	if err != nil {
		return nil, err
	}

	ret.LogList = logList

	// We grandfather in TiltStartTime from the old protocol,
	// because it tells the UI to reload.
	start, err := ptypes.TimestampProto(s.TiltStartTime)
	if err != nil {
		return nil, err
	}
	ret.TiltStartTime = start
	ret.IsComplete = true

	sortUIResources(ret.UiResources, s.ManifestDefinitionOrder)

	return ret, nil
}

// Create a view that only contains logs since the given checkpoint.
func LogUpdate(st store.RStore, checkpoint logstore.Checkpoint) (*proto_webview.View, error) {
	ret := &proto_webview.View{}

	s := st.RLockState()
	defer st.RUnlockState()
	logList, err := s.LogStore.ToLogList(checkpoint)
	if err != nil {
		return nil, err
	}

	ret.LogList = logList

	// We grandfather in TiltStartTime from the old protocol,
	// because it tells the UI to reload.
	start, err := ptypes.TimestampProto(s.TiltStartTime)
	if err != nil {
		return nil, err
	}
	ret.TiltStartTime = start

	return ret, nil
}

func sortUIResources(resources []*v1alpha1.UIResource, order []model.ManifestName) {
	resourceOrder := make(map[string]int, len(order))
	for i, name := range order {
		resourceOrder[name.String()] = i
	}
	resourceOrder[store.TiltfileManifestName.String()] = -1
	sort.Slice(resources, func(i, j int) bool {
		objI := resources[i]
		objJ := resources[j]
		orderI, hasI := resourceOrder[objI.Name]
		orderJ, hasJ := resourceOrder[objJ.Name]
		if !hasI {
			orderI = 1000
		}
		if !hasJ {
			orderJ = 1000
		}
		if orderI != orderJ {
			return orderI < orderJ
		}
		return objI.Name < objJ.Name
	})
}

// Converts EngineState into the public data model representation, a UISession.
func ToUISession(s store.EngineState) *v1alpha1.UISession {
	ret := &v1alpha1.UISession{
		ObjectMeta: metav1.ObjectMeta{
			Name: UISessionName,
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

// Converts an EngineState into a list of UIResources.
// The order of the list is non-deterministic.
func ToUIResourceList(state store.EngineState) ([]*v1alpha1.UIResource, error) {
	ret := make([]*v1alpha1.UIResource, 0, len(state.ManifestTargets)+1)
	ret = append(ret, TiltfileResourceProtoView(state))
	for i, mt := range state.Targets() {
		// Skip manifests that don't come from the tiltfile.
		if mt.Manifest.Source != model.ManifestSourceTiltfile {
			continue
		}

		r, err := toUIResource(mt, state)
		if err != nil {
			return nil, err
		}

		// The Tiltfile has Order -1, all "normal" resources have Order >= 1
		r.Status.Order = int32(i + 1)

		ret = append(ret, r)
	}
	return ret, nil
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

func TiltfileResourceProtoView(s store.EngineState) *v1alpha1.UIResource {
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
			Order:         -1,
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
		podID := k8s.PodID(pod.Name)
		r.Status.K8sResourceInfo = &v1alpha1.UIResourceKubernetes{
			PodName:            pod.Name,
			PodCreationTime:    pod.CreatedAt,
			PodUpdateStartTime: apis.NewTime(kState.UpdateStartTime[k8s.PodID(pod.Name)]),
			PodStatus:          pod.Status,
			PodStatusMessage:   strings.Join(pod.Errors, "\n"),
			AllContainersReady: store.AllPodContainersReady(pod),
			PodRestarts:        kState.VisiblePodContainerRestarts(podID),
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

package webview

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

func toAPITargetSpec(spec model.TargetSpec) (v1alpha1.UIResourceTargetSpec, error) {
	switch typ := spec.(type) {
	case model.ImageTarget:
		return v1alpha1.UIResourceTargetSpec{
			ID:            typ.ID().String(),
			Type:          v1alpha1.UIResourceTargetTypeImage,
			HasLiveUpdate: !typ.LiveUpdateInfo().Empty(),
		}, nil
	case model.DockerComposeTarget:
		return v1alpha1.UIResourceTargetSpec{
			ID:   typ.ID().String(),
			Type: v1alpha1.UIResourceTargetTypeDockerCompose,
		}, nil
	case model.K8sTarget:
		return v1alpha1.UIResourceTargetSpec{
			ID:   typ.ID().String(),
			Type: v1alpha1.UIResourceTargetTypeKubernetes,
		}, nil
	case model.LocalTarget:
		return v1alpha1.UIResourceTargetSpec{
			ID:   typ.ID().String(),
			Type: v1alpha1.UIResourceTargetTypeLocal,
		}, nil
	default:
		return v1alpha1.UIResourceTargetSpec{}, fmt.Errorf("unknown TargetSpec type %T for spec: '%v'", spec, spec)
	}
}

func ToAPITargetSpecs(specs []model.TargetSpec) ([]v1alpha1.UIResourceTargetSpec, error) {
	result := make([]v1alpha1.UIResourceTargetSpec, len(specs))
	for i, spec := range specs {
		protoSpec, err := toAPITargetSpec(spec)
		if err != nil {
			return nil, err
		}
		result[i] = protoSpec
	}

	return result, nil
}

func ToBuildRunning(br model.BuildRecord) *v1alpha1.UIBuildRunning {
	if br.Empty() {
		return nil
	}

	return &v1alpha1.UIBuildRunning{
		StartTime: metav1.NewMicroTime(br.StartTime),
		SpanID:    string(br.SpanID),
	}
}

func ToBuildTerminated(br model.BuildRecord, logStore *logstore.LogStore) v1alpha1.UIBuildTerminated {
	e := ""
	if br.Error != nil {
		e = br.Error.Error()
	}

	warnings := []string{}
	if br.SpanID != "" {
		warnings = logStore.Warnings(br.SpanID)
	}

	return v1alpha1.UIBuildTerminated{
		Error: e,
		// TODO(nick): Remove this, and compute it client-side.
		Warnings:       warnings,
		StartTime:      metav1.NewMicroTime(br.StartTime),
		FinishTime:     metav1.NewMicroTime(br.FinishTime),
		IsCrashRebuild: br.Reason.IsCrashOnly(),
		SpanID:         string(br.SpanID),
	}
}

func ToBuildsTerminated(brs []model.BuildRecord, logStore *logstore.LogStore) []v1alpha1.UIBuildTerminated {
	ret := make([]v1alpha1.UIBuildTerminated, len(brs))
	for i, br := range brs {
		ret[i] = ToBuildTerminated(br, logStore)
	}
	return ret
}

func ToAPILinks(lns []model.Link) []v1alpha1.UIResourceLink {
	ret := make([]v1alpha1.UIResourceLink, len(lns))
	for i, ln := range lns {
		ret[i] = v1alpha1.UIResourceLink{
			URL:  ln.URLString(),
			Name: ln.Name,
		}
	}
	return ret
}

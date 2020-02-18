package webview

// TODO(dmiller): delete these tests once StateToWebView is deleted
import (
	"fmt"
	"time"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/pkg/model"
	"github.com/windmilleng/tilt/pkg/model/logstore"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"

	proto_webview "github.com/windmilleng/tilt/pkg/webview"
)

func NewProtoDCResourceInfo(configPaths []string, status dockercompose.Status, cID container.ID, startTime time.Time) (*proto_webview.DCResourceInfo, error) {
	start, err := timeToProto(startTime)
	if err != nil {
		return nil, err
	}
	return &proto_webview.DCResourceInfo{
		ConfigPaths:     configPaths,
		ContainerStatus: string(status),
		ContainerID:     string(cID),
		StartTime:       start,
	}, nil
}

func timeToProto(t time.Time) (*timestamp.Timestamp, error) {
	ts, err := ptypes.TimestampProto(t)
	if err != nil {
		return nil, err
	}

	return ts, nil
}

func buildTypesToProtoUpdateTypes(bts []model.BuildType) ([]proto_webview.UpdateType, error) {
	result := make([]proto_webview.UpdateType, len(bts))
	for i, bt := range bts {
		protoType, err := buildTypeToProto(bt)
		if err != nil {
			return nil, err
		}
		result[i] = protoType
	}
	return result, nil
}

func buildTypeToProto(bt model.BuildType) (proto_webview.UpdateType, error) {
	switch bt {
	case model.BuildTypeImage:
		return proto_webview.UpdateType_UPDATE_TYPE_IMAGE, nil
	case model.BuildTypeLiveUpdate:
		return proto_webview.UpdateType_UPDATE_TYPE_LIVE_UPDATE, nil
	case model.BuildTypeDockerCompose:
		return proto_webview.UpdateType_UPDATE_TYPE_DOCKER_COMPOSE, nil
	case model.BuildTypeK8s:
		return proto_webview.UpdateType_UPDATE_TYPE_K8S, nil
	case model.BuildTypeLocal:
		return proto_webview.UpdateType_UPDATE_TYPE_LOCAL, nil
	default:
		return proto_webview.UpdateType_UPDATE_TYPE_UNSPECIFIED, fmt.Errorf("unknown build type '%v'", bt)
	}
}

func targetSpecToProto(spec model.TargetSpec) (proto_webview.TargetSpec, error) {
	switch typ := spec.(type) {
	case model.ImageTarget:
		return proto_webview.TargetSpec{
			Id:            typ.ID().String(),
			Type:          proto_webview.TargetType_TARGET_TYPE_IMAGE,
			HasLiveUpdate: !typ.LiveUpdateInfo().Empty(),
		}, nil
	case model.DockerComposeTarget:
		return proto_webview.TargetSpec{
			Id:   typ.ID().String(),
			Type: proto_webview.TargetType_TARGET_TYPE_DOCKER_COMPOSE,
		}, nil
	case model.K8sTarget:
		return proto_webview.TargetSpec{
			Id:   typ.ID().String(),
			Type: proto_webview.TargetType_TARGET_TYPE_K8S,
		}, nil
	case model.LocalTarget:
		return proto_webview.TargetSpec{
			Id:   typ.ID().String(),
			Type: proto_webview.TargetType_TARGET_TYPE_LOCAL,
		}, nil
	default:
		return proto_webview.TargetSpec{}, fmt.Errorf("unknown TargetSpec type %T for spec: '%v'", spec, spec)
	}
}

func TargetSpecsToProto(specs []model.TargetSpec) ([]*proto_webview.TargetSpec, error) {
	result := make([]*proto_webview.TargetSpec, len(specs))
	for i, spec := range specs {
		protoSpec, err := targetSpecToProto(spec)
		if err != nil {
			return nil, err
		}
		result[i] = &protoSpec
	}

	return result, nil
}

func ToProtoBuildRecord(br model.BuildRecord, logStore *logstore.LogStore) (*proto_webview.BuildRecord, error) {
	e := ""
	if br.Error != nil {
		e = br.Error.Error()
	}

	start, err := timeToProto(br.StartTime)
	if err != nil {
		return nil, err
	}
	finish, err := timeToProto(br.FinishTime)
	if err != nil {
		return nil, err
	}

	warnings := []string{}
	if br.SpanID != "" {
		warnings = logStore.Warnings(br.SpanID)
	}

	updateTypes, err := buildTypesToProtoUpdateTypes(br.BuildTypes)
	if err != nil {
		return nil, err
	}
	return &proto_webview.BuildRecord{
		Edits: br.Edits,
		Error: e,
		// TODO(nick): Remove this, and compute it client-side.
		Warnings:       warnings,
		StartTime:      start,
		FinishTime:     finish,
		UpdateTypes:    updateTypes,
		IsCrashRebuild: br.Reason.IsCrashOnly(),
		SpanId:         string(br.SpanID),
	}, nil
}

func ToProtoBuildRecords(brs []model.BuildRecord, logStore *logstore.LogStore) ([]*proto_webview.BuildRecord, error) {
	ret := make([]*proto_webview.BuildRecord, len(brs))
	for i, br := range brs {
		r, err := ToProtoBuildRecord(br, logStore)
		if err != nil {
			return nil, err
		}
		ret[i] = r
	}
	return ret, nil
}

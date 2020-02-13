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

func buildTypesToProto(bts []model.BuildType) ([]proto_webview.BuildType, error) {
	result := make([]proto_webview.BuildType, len(bts))
	for i, bt := range bts {
		webviewTyp, err := buildTypeToProto(bt)
		if err != nil {
			return nil, err
		}
		result[i] = webviewTyp
	}
	return result, nil
}

func buildTypeToProto(bt model.BuildType) (proto_webview.BuildType, error) {
	switch bt {
	case model.BuildTypeImage:
		return proto_webview.BuildType_IMAGE, nil
	case model.BuildTypeLiveUpdate:
		return proto_webview.BuildType_LIVE_UPDATE, nil
	case model.BuildTypeDockerCompose:
		return proto_webview.BuildType_DOCKER_COMPOSE, nil
	case model.BuildTypeK8s:
		return proto_webview.BuildType_K8S, nil
	case model.BuildTypeLocal:
		return proto_webview.BuildType_LOCAL, nil
	default:
		return proto_webview.BuildType_IMAGE, fmt.Errorf("unknown build type '%v'", bt)
	}
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

	types, err := buildTypesToProto(br.BuildTypes)
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
		BuildTypes:     types,
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

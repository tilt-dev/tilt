package webview

// TODO(dmiller): delete these tests once StateToWebView is deleted
import (
	"fmt"
	"time"

	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"

	proto_webview "github.com/tilt-dev/tilt/pkg/webview"
)

func timeToProto(t time.Time) (*timestamp.Timestamp, error) {
	ts, err := ptypes.TimestampProto(t)
	if err != nil {
		return nil, err
	}

	return ts, nil
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

	return &proto_webview.BuildRecord{
		Error: e,
		// TODO(nick): Remove this, and compute it client-side.
		Warnings:       warnings,
		StartTime:      start,
		FinishTime:     finish,
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

func ToProtoLinks(lns []model.Link) []*proto_webview.Link {
	ret := make([]*proto_webview.Link, len(lns))
	for i, ln := range lns {
		ret[i] = &proto_webview.Link{
			Url:  ln.URLString(),
			Name: ln.Name,
		}
	}
	return ret
}

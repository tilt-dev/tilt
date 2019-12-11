package webview

// TODO(dmiller): delete these tests once StateToWebView is deleted
import (
	"time"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/pkg/model"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"

	proto_webview "github.com/windmilleng/tilt/pkg/webview"
)

type ResourceInfoView interface {
	resourceInfoView()
	RuntimeLog() model.Log
	Status() string
}

type DCResourceInfo struct {
	ConfigPaths     []string             `json:"configPaths"`
	ContainerStatus dockercompose.Status `json:"containerStatus"`
	ContainerID     container.ID         `json:"containerID"`
	Log             model.Log            `json:"log"`
	StartTime       time.Time            `json:"startTime"`
}

func NewDCResourceInfo(configPaths []string, status dockercompose.Status, cID container.ID, log model.Log, startTime time.Time) DCResourceInfo {
	return DCResourceInfo{
		ConfigPaths:     configPaths,
		ContainerStatus: status,
		ContainerID:     cID,
		Log:             log,
		StartTime:       startTime,
	}
}

func NewProtoDCResourceInfo(configPaths []string, status dockercompose.Status, cID container.ID, log model.Log, startTime time.Time) (*proto_webview.DCResourceInfo, error) {
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

var _ ResourceInfoView = DCResourceInfo{}

func (DCResourceInfo) resourceInfoView()            {}
func (dcInfo DCResourceInfo) RuntimeLog() model.Log { return dcInfo.Log }
func (dcInfo DCResourceInfo) Status() string        { return string(dcInfo.ContainerStatus) }

type K8sResourceInfo struct {
	PodName            string    `json:"podName"`
	PodCreationTime    time.Time `json:"podCreationTime"`
	PodUpdateStartTime time.Time `json:"podUpdateStartTime"`
	PodStatus          string    `json:"podStatus"`
	PodStatusMessage   string    `json:"podStatusMessage"`
	AllContainersReady bool      `json:"allContainersReady"`
	PodRestarts        int       `json:"podRestarts"`
	PodLog             model.Log `json:"podLog"`
}

var _ ResourceInfoView = K8sResourceInfo{}

func (K8sResourceInfo) resourceInfoView()             {}
func (k8sInfo K8sResourceInfo) RuntimeLog() model.Log { return k8sInfo.PodLog }
func (k8sInfo K8sResourceInfo) Status() string {
	status := k8sInfo.PodStatus
	if status == "Running" && !k8sInfo.AllContainersReady {
		status = "Pending"
	}
	return status
}

type YAMLResourceInfo struct {
	K8sResources []string `json:"k8sResources"`
}

var _ ResourceInfoView = YAMLResourceInfo{}

func (YAMLResourceInfo) resourceInfoView()              {}
func (yamlInfo YAMLResourceInfo) RuntimeLog() model.Log { return model.NewLog("") }
func (yamlInfo YAMLResourceInfo) Status() string        { return "" }

// Local resources have no run time info, so it's all empty.
type LocalResourceInfo struct{}

var _ ResourceInfoView = LocalResourceInfo{}

func (LocalResourceInfo) resourceInfoView()     {}
func (LocalResourceInfo) RuntimeLog() model.Log { return model.NewLog("") }
func (LocalResourceInfo) Status() string        { return "" }

func timeToProto(t time.Time) (*timestamp.Timestamp, error) {
	ts, err := ptypes.TimestampProto(t)
	if err != nil {
		return nil, err
	}

	return ts, nil
}

func ToProtoBuildRecord(br model.BuildRecord) (*proto_webview.BuildRecord, error) {
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

	return &proto_webview.BuildRecord{
		Edits:          br.Edits,
		Error:          e,
		Warnings:       br.Warnings,
		StartTime:      start,
		FinishTime:     finish,
		IsCrashRebuild: br.Reason.IsCrashOnly(),
	}, nil
}

func ToProtoBuildRecords(brs []model.BuildRecord) ([]*proto_webview.BuildRecord, error) {
	ret := make([]*proto_webview.BuildRecord, len(brs))
	for i, br := range brs {
		r, err := ToProtoBuildRecord(br)
		if err != nil {
			return nil, err
		}
		ret[i] = r
	}
	return ret, nil
}

type RuntimeStatus string

const (
	RuntimeStatusOK            RuntimeStatus = "ok"
	RuntimeStatusPending       RuntimeStatus = "pending"
	RuntimeStatusError         RuntimeStatus = "error"
	RuntimeStatusNotApplicable RuntimeStatus = "not_applicable"
)

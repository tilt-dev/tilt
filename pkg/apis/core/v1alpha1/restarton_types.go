package v1alpha1

import v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// RestartOnSpec indicates the set of objects that can trigger a restart of this object.
type RestartOnSpec struct {
	// FileWatches that can trigger a restart.
	// +optional
	FileWatches []string `json:"fileWatches,omitempty" protobuf:"bytes,1,rep,name=fileWatches"`

	// UIButtons that can trigger a restart.
	// +optional
	UIButtons []string `json:"uiButtons,omitempty" protobuf:"bytes,2,rep,name=uiButtons"`
}

// StartOnSpec indicates the set of objects that can trigger a start/restart of this object.
type StartOnSpec struct {
	// StartAfter indicates that events before this time should be ignored.
	//
	// +optional
	StartAfter v1.Time `json:"startAfter,omitempty" protobuf:"bytes,1,opt,name=startAfter"`

	// UIButtons that can trigger a start/restart.
	UIButtons []string `json:"uiButtons" protobuf:"bytes,2,rep,name=uiButtons"`
}

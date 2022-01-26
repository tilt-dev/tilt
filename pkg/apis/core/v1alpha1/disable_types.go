package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Points at a thing that can control whether something is disabled
type DisableSource struct {
	// This DisableSource is controlled by a ConfigMap
	ConfigMap *ConfigMapDisableSource `json:"configMap,omitempty" protobuf:"bytes,2,opt,name=configMap"`
}

// Specifies a ConfigMap to control a DisableSource
type ConfigMapDisableSource struct {
	// The name of the ConfigMap
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`

	// The key where the enable/disable state is stored.
	Key string `json:"key" protobuf:"bytes,2,opt,name=key"`
}

type DisableStatus struct {
	// Whether this is currently disabled. Deprecated in favor of `State`.
	Disabled bool `json:"disabled" protobuf:"varint,1,opt,name=disabled"`
	// The last time this status was updated.
	LastUpdateTime metav1.Time `json:"lastUpdateTime" protobuf:"bytes,2,opt,name=lastUpdateTime"`
	// The reason this status was updated.
	Reason string `json:"reason" protobuf:"bytes,3,opt,name=reason"`
	// Whether this is currently disabled (if known)
	State DisableState `json:"state" protobuf:"bytes,4,opt,name=state,casttype=DisableState"`
}

// Indicates what is known about whether this is disabled.
// Possible values:
// "" - the status is not known
// "Enabled" - this is enabled
// "Disabled" - this is disabled
// "Error" - the status was not determined due to a potentially nonephemeral error (e.g., misconfiguration)
type DisableState string

const (
	DisableStatePending  DisableState = ""
	DisableStateEnabled  DisableState = "Enabled"
	DisableStateDisabled DisableState = "Disabled"
	DisableStateError    DisableState = "Error"
)

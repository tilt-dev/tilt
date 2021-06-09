package v1alpha1

// Selector for any Kubernetes-style API.
type ObjectSelector struct {

	// A regular expression apiVersion match.
	// +optional
	APIVersionRegexp string `json:"apiVersionRegexp,omitempty" protobuf:"bytes,1,opt,name=apiVersionRegexp"`

	// A regular expression kind match.
	// +optional
	KindRegexp string `json:"kindRegexp,omitempty" protobuf:"bytes,2,opt,name=kindRegexp"`

	// A regular expression name match.
	// +optional
	NameRegexp string `json:"nameRegexp,omitempty" protobuf:"bytes,3,opt,name=nameRegexp"`

	// A regular expression namespace match.
	// +optional
	NamespaceRegexp string `json:"namespaceRegexp,omitempty" protobuf:"bytes,4,opt,name=namespaceRegexp"`
}

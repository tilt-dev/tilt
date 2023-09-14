package v1alpha1

import (
	"context"
	"fmt"

	"github.com/distribution/reference"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type RegistryHosting struct {
	// Host documents the host (hostname and port) of the registry, as seen from
	// outside the cluster.
	//
	// This is the registry host that tools outside the cluster should push images
	// to.
	Host string `json:"host" yaml:"host" protobuf:"bytes,1,opt,name=host"`

	// HostFromClusterNetwork documents the host (hostname and port) of the
	// registry, as seen from networking inside the container pods.
	//
	// This is the registry host that tools running on pods inside the cluster
	// should push images to. If not set, then tools inside the cluster should
	// assume the local registry is not available to them.
	//
	// +optional
	HostFromClusterNetwork string `json:"hostFromClusterNetwork,omitempty" yaml:"hostFromClusterNetwork,omitempty" protobuf:"bytes,2,opt,name=hostFromClusterNetwork"`

	// HostFromContainerRuntime documents the host (hostname and port) of the
	// registry, as seen from the cluster's container runtime.
	//
	// When tools apply Kubernetes objects to the cluster, this host should be
	// used for image name fields. If not set, users of this field should use the
	// value of Host instead.
	//
	// Note that it doesn't make sense semantically to define this field, but not
	// define Host or HostFromClusterNetwork. That would imply a way to pull
	// images without a way to push images.
	//
	// +optional
	HostFromContainerRuntime string `json:"hostFromContainerRuntime,omitempty" yaml:"hostFromContainerRuntime,omitempty" protobuf:"bytes,3,opt,name=hostFromContainerRuntime"`

	// Help contains a URL pointing to documentation for users on how to set
	// up and configure a local registry.
	//
	// Tools can use this to nudge users to enable the registry. When possible,
	// the writer should use as permanent a URL as possible to prevent drift
	// (e.g., a version control SHA).
	//
	// When image pushes to a registry host specified in one of the other fields
	// fail, the tool should display this help URL to the user. The help URL
	// should contain instructions on how to diagnose broken or misconfigured
	// registries.
	//
	// +optional
	Help string `json:"help,omitempty" yaml:"help,omitempty" protobuf:"bytes,4,opt,name=help"`

	// SingleName uses a shared image name for _all_ Tilt-built images and
	// relies on tags to distinguish between logically distinct images.
	//
	// This is most commonly used with Amazon Elastic Container Registry (ECR),
	// which works differently than other image registries.
	//
	// An ECR host takes the form https://aws_account_id.dkr.ecr.region.amazonaws.com.
	// Each image name in that registry must be pre-created ಠ_ಠ and assigned
	// IAM permissions.
	// For example: https://aws_account_id.dkr.ecr.region.amazonaws.com/my-repo
	// (They call this a repo).
	//
	// For this reason, some users using ECR prefer to push all images to a
	// single image name (ECR repo).
	//
	// A recommended pattern here is to create a "personal" image repo for each
	// user during development.
	//
	// See:
	// https://docs.aws.amazon.com/AmazonECR/latest/userguide/Repositories.html
	// https://github.com/tilt-dev/tilt/issues/2419
	//
	// +optional
	SingleName string `json:"singleName,omitempty" protobuf:"bytes,5,opt,name=singleName"`
}

func (in *RegistryHosting) Validate(ctx context.Context) field.ErrorList {
	return in.validateAsSubfield(ctx, nil)
}

func (in *RegistryHosting) validateAsSubfield(_ context.Context, rootField *field.Path) field.ErrorList {
	var errors field.ErrorList
	if in.Host == "" {
		errors = append(errors, field.Required(rootField.Child("host"), ""))
	} else {
		if _, err := reference.ParseNamed(fmt.Sprintf("%s/%s", in.Host, "fake")); err != nil {
			errors = append(errors, field.Invalid(
				rootField.Child("host"),
				in.Host,
				err.Error()))
		}
	}

	if in.HostFromContainerRuntime != "" {
		if _, err := reference.ParseNamed(fmt.Sprintf("%s/%s", in.HostFromContainerRuntime, "fake")); err != nil {
			errors = append(errors, field.Invalid(
				rootField.Child("hostFromContainerRuntime"),
				in.HostFromContainerRuntime,
				err.Error()))
		}
	}

	return errors
}

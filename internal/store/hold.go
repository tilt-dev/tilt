package store

import (
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// We place a "hold" on a manifest if we can't build it
// because it's waiting on something.
type Hold struct {
	Reason HoldReason

	// Pointers to the internal data model we're holding for.
	HoldOn []model.TargetID

	// Pointers to the API objects we're holding for.
	OnRefs []v1alpha1.UIResourceStateWaitingOnRef
}

func (h Hold) String() string {
	return string(h.Reason)
}

type HoldReason string

const (
	HoldReasonNone                             HoldReason = ""
	HoldReasonTiltfileReload                   HoldReason = "tiltfile-reload"
	HoldReasonWaitingForUnparallelizableTarget HoldReason = "waiting-for-local"
	HoldReasonIsUnparallelizableTarget         HoldReason = "is-unparallelizable-local"
	HoldReasonWaitingForUncategorized          HoldReason = "waiting-for-uncategorized"
	HoldReasonBuildingComponent                HoldReason = "building-component"
	HoldReasonWaitingForDep                    HoldReason = "waiting-for-dep"
	HoldReasonWaitingForDeploy                 HoldReason = "waiting-for-deploy"

	// We're waiting for a reconciler to respond to the change,
	// but don't know yet what it's waiting on.
	HoldReasonReconciling HoldReason = "reconciling"

	// We're waiting on the cluster connection to be established.
	HoldReasonCluster HoldReason = "waiting-for-cluster"
)

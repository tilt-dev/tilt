package store

import "github.com/tilt-dev/tilt/pkg/model"

// We place a "hold" on a manifest if we can't build it
// because it's waiting on something.
type Hold struct {
	Reason HoldReason

	HoldOn []model.TargetID
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
	HoldReasonDisabled                         HoldReason = "disabled"
)

package store

// We place a "hold" on a manifest if we can't build it
// because it's waiting on something.
type Hold string

const (
	HoldNone                             Hold = ""
	HoldTiltfileReload                   Hold = "tiltfile-reload"
	HoldWaitingForUnparallelizableTarget Hold = "waiting-for-local"
	HoldIsUnparallelizableTarget         Hold = "is-unparallelizable-local"
	HoldWaitingForUncategorized          Hold = "waiting-for-uncategorized"
	HoldBuildingComponent                Hold = "building-component"
	HoldWaitingForDep                    Hold = "waiting-for-dep"
	HoldWaitingForDeploy                 Hold = "waiting-for-deploy"
	HoldDisabled                         Hold = "disabled"
)

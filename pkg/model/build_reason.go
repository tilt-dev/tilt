package model

import "strings"

type BuildReason int

const BuildReasonNone = BuildReason(0)

const (
	BuildReasonFlagChangedFiles BuildReason = 1 << iota
	BuildReasonFlagConfig

	// NOTE(nick): In live-update-v1, if a container had live-updated changed,
	// then crashed, we would automatically replace it with a fresh image.
	// This approach was difficult to reason about and sometimes led to infinite loops.
	// Users complained that it was too aggressive about doing an image build.
	//
	// In live-update-v2, the reconciler keeps track of how to bring crashing
	// containers up to date. Instead, we only kick off fresh image builds
	// if there's a new file change / trigger but the container has been
	// marked unrecoverable. So this build reason is obsolete.
	BuildReasonFlagCrashDeprecated

	BuildReasonFlagInit

	BuildReasonFlagTriggerWeb
	BuildReasonFlagTriggerCLI
	BuildReasonFlagTriggerHUD
	BuildReasonFlagTriggerUnknown

	// An external process called `tilt args`
	BuildReasonFlagTiltfileArgs

	// Suppose you have
	// manifestA with imageA depending on imageCommon
	// manifestB with imageB depending on imageCommon
	//
	// Building manifestA will mark imageB
	// with changed dependencies.
	BuildReasonFlagChangedDeps
)

func (r BuildReason) With(flag BuildReason) BuildReason {
	return r | flag
}

func (r BuildReason) Has(flag BuildReason) bool {
	return r&flag == flag
}

func (r BuildReason) HasTrigger() bool {
	for _, v := range triggerBuildReasons {
		if r.Has(v) {
			return true
		}
	}
	return false
}

func (r BuildReason) WithoutTriggers() BuildReason {
	result := int(r)
	for _, v := range triggerBuildReasons {
		if r.Has(v) {
			result -= int(v)
		}
	}
	return BuildReason(result)
}

var translations = map[BuildReason]string{
	BuildReasonFlagChangedFiles:    "Changed Files",
	BuildReasonFlagConfig:          "Config Changed",
	BuildReasonFlagCrashDeprecated: "Pod Crashed, Lost live_update Changes",
	BuildReasonFlagInit:            "Initial Build",
	BuildReasonFlagTriggerWeb:      "Web Trigger",
	BuildReasonFlagTriggerCLI:      "CLI Trigger",
	BuildReasonFlagTriggerHUD:      "HUD Trigger",
	BuildReasonFlagTriggerUnknown:  "Unknown Trigger",
	BuildReasonFlagTiltfileArgs:    "Tilt Args",
	BuildReasonFlagChangedDeps:     "Dependency Updated",
}

var triggerBuildReasons = []BuildReason{
	BuildReasonFlagTriggerWeb,
	BuildReasonFlagTriggerCLI,
	BuildReasonFlagTriggerHUD,
	BuildReasonFlagTriggerUnknown,
}

var allBuildReasons = []BuildReason{
	BuildReasonFlagInit,
	BuildReasonFlagChangedFiles,
	BuildReasonFlagConfig,
	BuildReasonFlagCrashDeprecated,
	BuildReasonFlagTriggerWeb,
	BuildReasonFlagTriggerCLI,
	BuildReasonFlagTriggerHUD,
	BuildReasonFlagChangedDeps,
	BuildReasonFlagTriggerUnknown,
	BuildReasonFlagTiltfileArgs,
}

func (r BuildReason) String() string {
	rs := []string{}

	// The trigger build reasons should never be used in conjunction with another
	// build reason, because it was explicitly specified by the user rather than implicit.
	for _, v := range triggerBuildReasons {
		if r.Has(v) {
			return translations[v]
		}
	}

	// The Init build reason should be listed alone too.
	if r.Has(BuildReasonFlagInit) {
		return translations[BuildReasonFlagInit]
	}

	// Use an array to iterate over the translations to ensure the iteration order
	// is consistent.
	for _, v := range allBuildReasons {
		if r.Has(v) {
			rs = append(rs, translations[v])
		}
	}
	return strings.Join(rs, " | ")
}

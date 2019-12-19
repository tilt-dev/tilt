package model

import "strings"

type BuildReason int

const BuildReasonNone = BuildReason(0)

const (
	BuildReasonFlagChangedFiles BuildReason = 1 << iota
	BuildReasonFlagConfig

	// See comments on NeedsRebuildFromCrash
	BuildReasonFlagCrash

	BuildReasonFlagInit
)

func (r BuildReason) With(flag BuildReason) BuildReason {
	return r | flag
}

func (r BuildReason) Has(flag BuildReason) bool {
	return r&flag == flag
}

func (r BuildReason) IsCrashOnly() bool {
	return r == BuildReasonFlagCrash
}

var translations = map[BuildReason]string{
	BuildReasonFlagChangedFiles: "changed files",
	BuildReasonFlagConfig:       "config changed",
	BuildReasonFlagCrash:        "pod crashed, lost live_update changes",
	BuildReasonFlagInit:         "initial build",
}

var allBuildReasons = []BuildReason{
	BuildReasonFlagInit,
	BuildReasonFlagChangedFiles,
	BuildReasonFlagConfig,
	BuildReasonFlagCrash,
}

func (r BuildReason) String() string {
	rs := []string{}

	// Use an array to iterate over the translations to ensure the iteration order
	// is consistent.
	for _, v := range allBuildReasons {
		if r.Has(v) {
			rs = append(rs, translations[v])
		}
	}
	return strings.Join(rs, " | ")
}

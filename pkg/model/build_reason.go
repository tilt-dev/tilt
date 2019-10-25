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
	BuildReasonFlagCrash:        "pod crashed",
	BuildReasonFlagInit:         "tilt up",
}

func (r BuildReason) String() string {
	rs := []string{}

	for k, v := range translations {
		if r.Has(k) {
			rs = append(rs, v)
		}
	}
	return strings.Join(rs, " | ")
}

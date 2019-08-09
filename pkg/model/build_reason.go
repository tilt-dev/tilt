package model

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

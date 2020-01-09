package model

type RuntimeStatus string

const (
	RuntimeStatusUnknown       RuntimeStatus = "unknown"
	RuntimeStatusOK            RuntimeStatus = "ok"
	RuntimeStatusPending       RuntimeStatus = "pending"
	RuntimeStatusError         RuntimeStatus = "error"
	RuntimeStatusNotApplicable RuntimeStatus = "not_applicable"
)

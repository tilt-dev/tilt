package model

import "time"

// Inject the flag-specified CI timeout.
type CITimeoutFlag time.Duration

const CITimeoutDefault = 30 * time.Minute

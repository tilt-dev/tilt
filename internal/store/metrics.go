package store

// User-specified presets for creating metrics.
type MetricsMode string

const MetricsNone MetricsMode = ""
const MetricsLocal MetricsMode = "local"
const MetricsProd MetricsMode = "prod"

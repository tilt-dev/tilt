package store

// How metrics are served to the user.
type MetricsMode string

const MetricsNone = MetricsMode("")
const MetricsLocal = MetricsMode("local")
const MetricsProd = MetricsMode("prod")

type MetricsServing struct {
	Mode        MetricsMode
	GrafanaHost string
}

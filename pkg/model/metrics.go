package model

import "time"

var DefaultReportingPeriod = 5 * time.Minute

// Metrics settings generally map to exporter options
// https://pkg.go.dev/contrib.go.opencensus.io/exporter/ocagent?tab=doc#ExporterOption
type MetricsSettings struct {
	Enabled  bool
	Address  string
	Insecure bool

	// How often Tilt reports its metrics. Useful for testing.
	// https://pkg.go.dev/go.opencensus.io/stats/view?tab=doc#SetReportingPeriod
	ReportingPeriod time.Duration
}

func DefaultMetricsSettings() MetricsSettings {
	return MetricsSettings{
		Enabled:         false,
		Address:         "opentelemetry.tilt.dev:443",
		ReportingPeriod: DefaultReportingPeriod,
	}
}

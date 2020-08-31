package model

// Metrics settings generally map to exporter options
// https://pkg.go.dev/contrib.go.opencensus.io/exporter/ocagent?tab=doc#ExporterOption
type MetricsSettings struct {
	Enabled  bool
	Address  string
	Insecure bool
}

// Package exptel is a fork of core Span types from opentelemetry-go v0.2.0.
//
// https://github.com/open-telemetry/opentelemetry-go/tree/v0.2.0
//
// These are used to maintain serialization compatibility for the `experimental_telemetry_cmd` output,
// which uses the JSON serialization of these directly. The canonical JSON serialization for OTLP is
// not standardized as of Sept 2021 and was removed as a requirement for v1.0 GA.
//
// See https://github.com/open-telemetry/opentelemetry-proto/issues/230.
package exptel

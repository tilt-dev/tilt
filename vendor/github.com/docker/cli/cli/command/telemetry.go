package command

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const exportTimeout = 50 * time.Millisecond

// TracerProvider is an extension of the trace.TracerProvider interface for CLI programs.
type TracerProvider interface {
	trace.TracerProvider
	ForceFlush(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

// MeterProvider is an extension of the metric.MeterProvider interface for CLI programs.
type MeterProvider interface {
	metric.MeterProvider
	ForceFlush(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

// TelemetryClient provides the methods for using OTEL tracing or metrics.
type TelemetryClient interface {
	// Resource returns the OTEL Resource configured with this TelemetryClient.
	// This resource may be created lazily, but the resource should be the same
	// each time this function is invoked.
	Resource() *resource.Resource

	// TracerProvider returns the currently initialized TracerProvider. This TracerProvider will be configured
	// with the default tracing components for a CLI program
	TracerProvider() trace.TracerProvider

	// MeterProvider returns the currently initialized MeterProvider. This MeterProvider will be configured
	// with the default metric components for a CLI program
	MeterProvider() metric.MeterProvider
}

func (cli *DockerCli) Resource() *resource.Resource {
	return cli.res.Get()
}

func (*DockerCli) TracerProvider() trace.TracerProvider {
	return otel.GetTracerProvider()
}

func (*DockerCli) MeterProvider() metric.MeterProvider {
	return otel.GetMeterProvider()
}

// WithResourceOptions configures additional options for the default resource. The default
// resource will continue to include its default options.
func WithResourceOptions(opts ...resource.Option) CLIOption {
	return func(cli *DockerCli) error {
		cli.res.AppendOptions(opts...)
		return nil
	}
}

// WithResource overwrites the default resource and prevents its creation.
func WithResource(res *resource.Resource) CLIOption {
	return func(cli *DockerCli) error {
		cli.res.Set(res)
		return nil
	}
}

type telemetryResource struct {
	res  *resource.Resource
	opts []resource.Option
	once sync.Once
}

func (r *telemetryResource) Set(res *resource.Resource) {
	r.res = res
}

func (r *telemetryResource) Get() *resource.Resource {
	r.once.Do(r.init)
	return r.res
}

func (r *telemetryResource) init() {
	if r.res != nil {
		r.opts = nil
		return
	}

	opts := append(defaultResourceOptions(), r.opts...)
	res, err := resource.New(context.Background(), opts...)
	if err != nil {
		otel.Handle(err)
	}
	r.res = res

	// Clear the resource options since they'll never be used again and to allow
	// the garbage collector to retrieve that memory.
	r.opts = nil
}

// createGlobalMeterProvider creates a new MeterProvider from the initialized DockerCli struct
// with the given options and sets it as the global meter provider
func (cli *DockerCli) createGlobalMeterProvider(ctx context.Context, opts ...sdkmetric.Option) {
	allOpts := make([]sdkmetric.Option, 0, len(opts)+2)
	allOpts = append(allOpts, sdkmetric.WithResource(cli.Resource()))
	allOpts = append(allOpts, dockerMetricExporter(ctx, cli)...)
	allOpts = append(allOpts, opts...)
	mp := sdkmetric.NewMeterProvider(allOpts...)
	otel.SetMeterProvider(mp)
}

// createGlobalTracerProvider creates a new TracerProvider from the initialized DockerCli struct
// with the given options and sets it as the global tracer provider
func (cli *DockerCli) createGlobalTracerProvider(ctx context.Context, opts ...sdktrace.TracerProviderOption) {
	allOpts := make([]sdktrace.TracerProviderOption, 0, len(opts)+2)
	allOpts = append(allOpts, sdktrace.WithResource(cli.Resource()))
	allOpts = append(allOpts, dockerSpanExporter(ctx, cli)...)
	allOpts = append(allOpts, opts...)
	tp := sdktrace.NewTracerProvider(allOpts...)
	otel.SetTracerProvider(tp)
}

func defaultResourceOptions() []resource.Option {
	return []resource.Option{
		resource.WithDetectors(serviceNameDetector{}),
		resource.WithAttributes(
			// Use a unique instance id so OTEL knows that each invocation
			// of the CLI is its own instance. Without this, downstream
			// OTEL processors may think the same process is restarting
			// continuously.
			semconv.ServiceInstanceID(uuid.NewString()),
		),
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
	}
}

func (r *telemetryResource) AppendOptions(opts ...resource.Option) {
	if r.res != nil {
		return
	}
	r.opts = append(r.opts, opts...)
}

type serviceNameDetector struct{}

func (serviceNameDetector) Detect(ctx context.Context) (*resource.Resource, error) {
	return resource.StringDetector(
		semconv.SchemaURL,
		semconv.ServiceNameKey,
		func() (string, error) {
			return filepath.Base(os.Args[0]), nil
		},
	).Detect(ctx)
}

// cliReader is an implementation of Reader that will automatically
// report to a designated Exporter when Shutdown is called.
type cliReader struct {
	sdkmetric.Reader
	exporter sdkmetric.Exporter
}

func newCLIReader(exp sdkmetric.Exporter) sdkmetric.Reader {
	reader := sdkmetric.NewManualReader(
		sdkmetric.WithTemporalitySelector(deltaTemporality),
	)
	return &cliReader{
		Reader:   reader,
		exporter: exp,
	}
}

func (r *cliReader) Shutdown(ctx context.Context) error {
	// Place a pretty tight constraint on the actual reporting.
	// We don't want CLI metrics to prevent the CLI from exiting
	// so if there's some kind of issue we need to abort pretty
	// quickly.
	ctx, cancel := context.WithTimeout(ctx, exportTimeout)
	defer cancel()

	return r.ForceFlush(ctx)
}

func (r *cliReader) ForceFlush(ctx context.Context) error {
	var rm metricdata.ResourceMetrics
	if err := r.Reader.Collect(ctx, &rm); err != nil {
		return err
	}

	return r.exporter.Export(ctx, &rm)
}

// deltaTemporality sets the Temporality of every instrument to delta.
//
// This isn't really needed since we create a unique resource on each invocation,
// but it can help with cardinality concerns for downstream processors since they can
// perform aggregation for a time interval and then discard the data once that time
// period has passed. Cumulative temporality would imply to the downstream processor
// that they might receive a successive point and they may unnecessarily keep state
// they really shouldn't.
func deltaTemporality(_ sdkmetric.InstrumentKind) metricdata.Temporality {
	return metricdata.DeltaTemporality
}

// resourceAttributesEnvVar is the name of the envvar that includes additional
// resource attributes for OTEL as defined in the [OpenTelemetry specification].
//
// [OpenTelemetry specification]: https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/#general-sdk-configuration
const resourceAttributesEnvVar = "OTEL_RESOURCE_ATTRIBUTES"

func filterResourceAttributesEnvvar() {
	if v := os.Getenv(resourceAttributesEnvVar); v != "" {
		if filtered := filterResourceAttributes(v); filtered != "" {
			_ = os.Setenv(resourceAttributesEnvVar, filtered)
		} else {
			_ = os.Unsetenv(resourceAttributesEnvVar)
		}
	}
}

// dockerCLIAttributePrefix is the prefix for any docker cli OTEL attributes.
// When updating, make sure to also update the copy in cli-plugins/manager.
//
// TODO(thaJeztah): move telemetry-related code to an (internal) package to reduce dependency on cli/command in cli-plugins, which has too many imports.
const dockerCLIAttributePrefix = "docker.cli."

func filterResourceAttributes(s string) string {
	if trimmed := strings.TrimSpace(s); trimmed == "" {
		return trimmed
	}

	pairs := strings.Split(s, ",")
	elems := make([]string, 0, len(pairs))
	for _, p := range pairs {
		k, _, found := strings.Cut(p, "=")
		if !found {
			// Do not interact with invalid otel resources.
			elems = append(elems, p)
			continue
		}

		// Skip attributes that have our docker.cli prefix.
		if strings.HasPrefix(k, dockerCLIAttributePrefix) {
			continue
		}
		elems = append(elems, p)
	}
	return strings.Join(elems, ",")
}

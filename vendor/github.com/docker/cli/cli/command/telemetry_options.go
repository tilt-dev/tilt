package command

// WithEnableGlobalMeterProvider configures the DockerCli to create a new
// MeterProvider from the initialized DockerCli struct, and set it as
// the global meter provider.
//
// WARNING: For internal use, don't depend on this.
func WithEnableGlobalMeterProvider() CLIOption {
	return func(cli *DockerCli) error {
		cli.enableGlobalMeter = true
		return nil
	}
}

// WithEnableGlobalTracerProvider configures the DockerCli to create a new
// TracerProvider from the initialized DockerCli struct, and set it as
// the global tracer provider.
//
// WARNING: For internal use, don't depend on this.
func WithEnableGlobalTracerProvider() CLIOption {
	return func(cli *DockerCli) error {
		cli.enableGlobalTracer = true
		return nil
	}
}

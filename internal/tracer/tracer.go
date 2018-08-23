package tracer

import (
	opentracing "github.com/opentracing/opentracing-go"
	config "github.com/uber/jaeger-client-go/config"
)

func Init() error {
	cfg := &config.Configuration{
		Sampler: &config.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
	}
	// TODO(dmiller) log output to a file?
	tracer, _, err := cfg.New("tilt")
	if err != nil {
		return err
	}
	opentracing.SetGlobalTracer(tracer)

	return nil
}

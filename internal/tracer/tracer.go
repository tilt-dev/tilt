package tracer

import (
	"io"

	opentracing "github.com/opentracing/opentracing-go"
	config "github.com/uber/jaeger-client-go/config"
)

func Init() (io.Closer, error) {
	cfg := &config.Configuration{
		Sampler: &config.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
		ServiceName: "tilt",
	}
	// TODO(dmiller) log output to a file?
	tracer, closer, err := cfg.NewTracer()
	if err != nil {
		return nil, err
	}
	opentracing.SetGlobalTracer(tracer)

	return closer, nil
}

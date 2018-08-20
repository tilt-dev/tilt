package tracer

import (
	opentracing "github.com/opentracing/opentracing-go"
	jaeger "github.com/uber/jaeger-client-go"
	config "github.com/uber/jaeger-client-go/config"
)

func Init() error {
	cfg := &config.Configuration{
		Sampler: &config.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
		Reporter: &config.ReporterConfig{
			LogSpans: true,
		},
	}
	tracer, _, err := cfg.New("tilt", config.Logger(jaeger.StdLogger))
	if err != nil {
		return err
	}
	opentracing.SetGlobalTracer(tracer)

	return nil
}

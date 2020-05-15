package tracer

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/pkg/errors"

	"github.com/lightstep/lightstep-tracer-go"
	"github.com/opentracing/opentracing-go"
	zipkin "github.com/openzipkin/zipkin-go-opentracing"
	jaeger "github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"

	"github.com/tilt-dev/tilt/pkg/logger"
)

const windmillTracerHostPort = "opentracing.windmill.build:9411"

type TracerBackend int

const (
	Windmill TracerBackend = iota
	Lightstep
	Jaeger
)

type zipkinLogger struct {
	ctx context.Context
}

func (zl zipkinLogger) Log(keyvals ...interface{}) error {
	logger.Get(zl.ctx).Debugf("%v", keyvals)
	return nil
}

var _ zipkin.Logger = zipkinLogger{}

func Init(ctx context.Context, tracer TracerBackend) (func() error, error) {
	switch tracer {
	case Windmill:
		return initWindmillZipkin(ctx)
	case Lightstep:
		return initLightStep(ctx)
	case Jaeger:
		return initJaeger(ctx)
	default:
		return nil, fmt.Errorf("Init: Invalid Tracer backend: %d", tracer)
	}
}

func TraceID(ctx context.Context) (string, error) {
	spanContext := opentracing.SpanFromContext(ctx)
	if spanContext == nil {
		return "", errors.New("cannot get traceid - there is no span context")
	}
	switch t := spanContext.Context().(type) {
	case zipkin.SpanContext:
		return t.TraceID.ToHex(), nil
	case lightstep.SpanContext:
		return string(t.TraceID), nil
	case jaeger.SpanContext:
		return t.TraceID().String(), nil
	default:
		return "", errors.New("cannot get traceid - unknown span type")
	}
}

// TagStrToMap converts a user-passed string of tags of the form `key1=val1,key2=val2` to a map.
func TagStrToMap(tagStr string) map[string]string {
	if tagStr == "" {
		return nil
	}

	res := make(map[string]string)
	pairs := strings.Split(tagStr, ",")
	for _, p := range pairs {
		elems := strings.Split(strings.TrimSpace(p), "=")
		if len(elems) != 2 {
			log.Printf("got malformed trace tag: %s", p)
			continue
		}
		res[elems[0]] = elems[1]
	}
	return res
}

func StringToTracerBackend(s string) (TracerBackend, error) {
	switch s {
	case "windmill":
		return Windmill, nil
	case "lightstep":
		return Lightstep, nil
	case "jaeger":
		return Jaeger, nil
	default:
		return Windmill, fmt.Errorf("Invalid Tracer backend: %s", s)
	}
}

func initWindmillZipkin(ctx context.Context) (func() error, error) {
	collector, err := zipkin.NewHTTPCollector(fmt.Sprintf("http://%s/api/v1/spans", windmillTracerHostPort), zipkin.HTTPLogger(zipkinLogger{ctx}))

	if err != nil {
		return nil, errors.Wrap(err, "unable to create zipkin collector")
	}

	recorder := zipkin.NewRecorder(collector, true, "0.0.0.0:0", "tilt")
	tracer, err := zipkin.NewTracer(recorder)

	if err != nil {
		return nil, errors.Wrap(err, "unable to create tracer")
	}

	opentracing.SetGlobalTracer(tracer)

	return collector.Close, nil
}

func initLightStep(ctx context.Context) (func() error, error) {
	token, ok := os.LookupEnv("LIGHTSTEP_ACCESS_TOKEN")
	if !ok {
		return nil, fmt.Errorf("No token found in the LIGHTSTEP_ACCESS_TOKEN environment variable")
	}
	lightstepTracer := lightstep.NewTracer(lightstep.Options{
		AccessToken: token,
	})

	opentracing.SetGlobalTracer(lightstepTracer)

	close := func() error {
		lightstepTracer.Close(context.Background())
		return nil
	}
	return close, nil
}

func initJaeger(ctx context.Context) (func() error, error) {
	cfg := jaegercfg.Configuration{
		Sampler: &jaegercfg.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
	}
	closer, err := cfg.InitGlobalTracer("tilt")
	return closer.Close, err
}

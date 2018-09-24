package tracer

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/windmilleng/tilt/internal/logger"

	"github.com/pkg/errors"

	"github.com/opentracing/opentracing-go"
	zipkin "github.com/openzipkin/zipkin-go-opentracing"
)

const windmillTracerHostPort = "opentracing.windmill.build:9411"

type zipkinLogger struct {
	ctx context.Context
}

func (zl zipkinLogger) Log(keyvals ...interface{}) error {
	logger.Get(zl.ctx).Debugf("%v", keyvals)
	return nil
}

var _ zipkin.Logger = zipkinLogger{}

func Init(ctx context.Context) (func() error, error) {
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

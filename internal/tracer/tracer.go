package tracer

import (
	"io"
	"log"
	"strings"

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

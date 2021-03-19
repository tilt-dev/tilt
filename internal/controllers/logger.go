package controllers

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-logr/logr"

	"github.com/tilt-dev/tilt/pkg/logger"
)

// logrFacade is an implementation of logr.Logger that uses a Tilt logger.
//
// This is not meant as a generic implementation, but specifically for usage by controller-runtime internals,
// which expect a logr.Logger. As a result, everything is coerced to logger.DebugLvl because from the Tilt
// perspective, this is all internal system logging.
type logrFacade struct {
	name   string
	logger logger.Logger
	v      int
	//writer io.Writer
	values string
}

func newLogrFacade(l logger.Logger, name string) logrFacade {
	return logrFacade{
		name:   name,
		logger: l,
	}
}

func (l logrFacade) Enabled() bool {
	return l.logger.Level().ShouldDisplay(logger.DebugLvl)
}

func (l logrFacade) Info(msg string, keysAndValues ...interface{}) {
	l.log(msg, keysAndValues...)
}

func (l logrFacade) Error(err error, msg string, keysAndValues ...interface{}) {
	l.log(msg, append([]interface{}{logger.Red(l.logger).Sprint("error"), err}, keysAndValues...)...)
}

// V will be propagated and included in the log message if non-zero, but is currently not used for filtering.
//
// It is output so that if we determine that controller-runtime is outputting excessively verbose logs at high
// levels, we can add a filter to drop beyond some max level. As of March 2021, it seems to mostly log at the
// default level, so this is not a concern.
func (l logrFacade) V(level int) logr.Logger {
	l.v += level
	return l
}

func (l logrFacade) WithValues(keysAndValues ...interface{}) logr.Logger {
	newValues := flatten(keysAndValues...)
	if l.values != "" {
		l.values = l.values + " " + newValues
	} else {
		l.values = newValues
	}
	return l
}

func (l logrFacade) WithName(name string) logr.Logger {
	if l.name != "" {
		l.name = l.name + "." + name
	} else {
		l.name = name
	}
	return l
}

func (l logrFacade) log(msg string, keysAndValues ...interface{}) {
	var b bytes.Buffer
	b.WriteString(msg)
	if l.name != "" {
		// treat the logger name like a k-v
		b.WriteRune(' ')
		b.WriteString("logger=")
		b.WriteString(l.name)
	}
	if l.v > 0 {
		b.WriteRune(' ')
		b.WriteString("v=")
		b.WriteString(strconv.Itoa(l.v))
	}
	if l.values != "" {
		b.WriteRune(' ')
		b.WriteString(l.values)
	}
	kvs := flatten(keysAndValues...)
	if kvs != "" {
		b.WriteRune(' ')
		b.WriteString(kvs)
	}
	b.WriteRune('\n')
	l.logger.Write(logger.DebugLvl, b.Bytes())
}

func flatten(keysAndValues ...interface{}) string {
	var sb strings.Builder
	for i := 0; i < len(keysAndValues); i += 2 {
		if i > 0 {
			sb.WriteRune(' ')
		}
		key, ok := keysAndValues[i].(string)
		if !ok {
			continue
		}
		var value string
		if i+1 < len(keysAndValues) {
			switch v := keysAndValues[i+1].(type) {
			case string:
				value = v
			case fmt.Stringer:
				value = v.String()
			case error:
				value = v.Error()
			default:
				value = fmt.Sprint(v)
			}
		}
		sb.WriteString(quote(key))
		sb.WriteRune('=')
		sb.WriteString(quote(value))
	}
	return sb.String()
}

func quote(v string) string {
	if strings.ContainsAny(v, ` "`) {
		return fmt.Sprintf("%q", v)
	}
	return v
}

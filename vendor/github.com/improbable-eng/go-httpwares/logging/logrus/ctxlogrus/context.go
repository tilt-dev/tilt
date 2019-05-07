package ctxlogrus

import (
	"github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

type ctxLoggerMarker struct{}

type ctxLogger struct {
	logger *logrus.Entry
	fields logrus.Fields
}

var (
	ctxLoggerKey = &ctxLoggerMarker{}
)

// Extract takes the logrus.Entry from the context.
//
// The logger will have fields pre-populated that had been present when adding the logger to the context. Fields can
// also be added using AddFields.
//
// If the has had no logrus.Entry set a no-op logrus.Entry is returned. This makes it safe to use regardless.
func Extract(ctx context.Context) *logrus.Entry {
	l, ok := ctx.Value(ctxLoggerKey).(*ctxLogger)
	if !ok || l == nil {
		return logrus.NewEntry(nullLogger)
	}

	fields := logrus.Fields{}
	for k, v := range l.fields {
		fields[k] = v
	}

	for k, v := range grpc_ctxtags.Extract(ctx).Values() {
		fields[k] = v
	}

	return l.logger.WithFields(fields)
}

// AddFields will add or override fields to context for use when extracting a logger from the context.
func AddFields(ctx context.Context, fields logrus.Fields) {
	l, ok := ctx.Value(ctxLoggerKey).(*ctxLogger)
	if !ok || l == nil {
		return
	}
	for k, v := range fields {
		l.fields[k] = v
	}
}

// ToContext sets a logrus logger on the context, which can then obtained by Extract.
// this will override any logger or fields that are already on the context with the
// logger that has been passed in.
func ToContext(ctx context.Context, entry *logrus.Entry) context.Context {
	l := &ctxLogger{
		logger: entry,
		fields: logrus.Fields{},
	}
	return context.WithValue(ctx, ctxLoggerKey, l)
}

package synclet

import (
	"context"
	"fmt"

	"github.com/windmilleng/tilt/internal/synclet/proto"
	"github.com/windmilleng/tilt/pkg/logger"
)

func protoLogLevelToLevel(protoLevel proto.LogLevel) logger.Level {
	switch protoLevel {
	case proto.LogLevel_INFO:
		return logger.InfoLvl
	case proto.LogLevel_VERBOSE:
		return logger.VerboseLvl
	case proto.LogLevel_DEBUG:
		return logger.DebugLvl
	default:
		// the server returned a log level that we don't recognize - err on the side of caution and return
		// the minimum log level to ensure that all output is printed
		return logger.NoneLvl
	}
}

func logLevelToProto(level logger.Level) (proto.LogLevel, error) {
	switch level {
	case logger.InfoLvl:
		return proto.LogLevel_INFO, nil
	case logger.VerboseLvl:
		return proto.LogLevel_VERBOSE, nil
	case logger.DebugLvl:
		return proto.LogLevel_DEBUG, nil
	default:
		return proto.LogLevel_INFO, fmt.Errorf("unknown log level '%v'", level)
	}
}

func newLogStyle(ctx context.Context) (*proto.LogStyle, error) {
	l := logger.Get(ctx)
	supportsColor := l.SupportsColor()
	level, err := logLevelToProto(l.Level())
	if err != nil {
		return nil, err
	}
	return &proto.LogStyle{
		ColorsEnabled: supportsColor,
		Level:         level,
	}, nil
}

func makeContext(ctx context.Context, logStyle *proto.LogStyle, f func(m *proto.LogMessage) error) (context.Context, error) {
	writeLog := func(level logger.Level, fields logger.Fields, bytes []byte) error {
		protoLevel, err := logLevelToProto(level)
		if err != nil {
			return err
		}

		logMessage := &proto.LogMessage{Level: protoLevel, Message: bytes}
		return f(logMessage)
	}

	level := protoLogLevelToLevel(logStyle.Level)
	l := logger.NewFuncLogger(logStyle.ColorsEnabled, level, writeLog)

	return logger.WithLogger(ctx, l), nil
}

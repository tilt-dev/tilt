package logging

import (
	grpcLogrus "github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

func NewUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return grpcLogrus.UnaryServerInterceptor(Global())
}

func NewStreamServerInterceptor() grpc.StreamServerInterceptor {
	return grpcLogrus.StreamServerInterceptor(Global())
}

func NewUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	// The default is to log all OK requests as DEBUG.
	// https://github.com/grpc-ecosystem/go-grpc-middleware/blob/master/logging/logrus/options.go#L120
	return grpcLogrus.UnaryClientInterceptor(Global())
}

func NewStreamClientInterceptor() grpc.StreamClientInterceptor {
	return grpcLogrus.StreamClientInterceptor(Global())
}

func NewDaemonUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return grpcLogrus.UnaryClientInterceptor(Global(), grpcLogrus.WithLevels(daemonClientCodeToLevel))
}

func NewDaemonStreamClientInterceptor() grpc.StreamClientInterceptor {
	return grpcLogrus.StreamClientInterceptor(Global(), grpcLogrus.WithLevels(daemonClientCodeToLevel))
}

func daemonClientCodeToLevel(code codes.Code) logrus.Level {
	level := grpcLogrus.DefaultClientCodeToLevel(code)
	if level == logrus.DebugLevel {
		return logrus.InfoLevel
	}
	return level
}

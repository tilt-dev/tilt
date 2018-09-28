package options

import (
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"
)

// 1024 MB
const MaxMsgSize int = 1024 * 1024 * 1024

func MaxMsgDial() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithDefaultCallOptions(
			grpc.MaxCallSendMsgSize(MaxMsgSize),
			grpc.MaxCallRecvMsgSize(MaxMsgSize),
		),
	}
}

func TracingInterceptorsDial(t opentracing.Tracer) []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithUnaryInterceptor(otgrpc.OpenTracingClientInterceptor(t)),
		grpc.WithStreamInterceptor(otgrpc.OpenTracingStreamClientInterceptor(t)),
	}
}

func MaxMsgServer() []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.MaxRecvMsgSize(MaxMsgSize),
		grpc.MaxSendMsgSize(MaxMsgSize),
	}
}

func TracingInterceptorsServer(t opentracing.Tracer) []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.UnaryInterceptor(otgrpc.OpenTracingServerInterceptor(t)),
		grpc.StreamInterceptor(otgrpc.OpenTracingStreamServerInterceptor(t)),
	}
}

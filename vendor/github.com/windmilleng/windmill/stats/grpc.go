package stats

import (
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

func NewUnaryServerInterceptor(reporter StatsReporter) grpc.UnaryServerInterceptor {
	f := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		startTime := time.Now()
		res, err := handler(ctx, req)
		go finishGRPC(reporter, startTime, info.FullMethod, err)
		return res, err
	}
	return grpc.UnaryServerInterceptor(f)
}

func NewStreamServerInterceptor(reporter StatsReporter) grpc.StreamServerInterceptor {
	f := func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		startTime := time.Now()
		err := handler(srv, ss)
		go finishGRPC(reporter, startTime, info.FullMethod, err)
		return err
	}
	return grpc.StreamServerInterceptor(f)
}

func finishGRPC(reporter StatsReporter, startTime time.Time, methodName string, err error) {
	duration := time.Now().Sub(startTime)
	code := codes.OK
	if err != nil {
		code = grpc.Code(err)
	}
	tags := map[string]string{
		"method": methodName,
		"status": code.String(),
	}

	reporter.Incr("grpc.response", tags, 1)
	reporter.Timing("grpc.responseTime", duration, tags, 1)
}

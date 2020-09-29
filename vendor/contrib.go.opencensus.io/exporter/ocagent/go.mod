module contrib.go.opencensus.io/exporter/ocagent

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1 // this is to match the version used in census-instrumentation/opencensus-service
	github.com/golang/protobuf v1.4.2
	github.com/google/go-cmp v0.4.0
	github.com/grpc-ecosystem/grpc-gateway v1.14.6 // indirect
	go.opencensus.io v0.22.3
	golang.org/x/net v0.0.0-20200520182314-0ba52f642ac2 // indirect
	golang.org/x/sys v0.0.0-20200523222454-059865788121 // indirect
	google.golang.org/api v0.25.0
	google.golang.org/genproto v0.0.0-20200527145253-8367513e4ece // indirect
	google.golang.org/grpc v1.29.1
)

go 1.13

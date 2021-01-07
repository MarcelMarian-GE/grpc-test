module testgrpc.com/grpctest

go 1.13

replace testgrpc.com/grpcservice => ./testgrpc.com/grpcservice

replace testgrpc.com/mqttapi => ./testgrpc.com/mqttapi

require (
	github.com/eclipse/paho.mqtt.golang v1.3.0
	github.com/gogo/protobuf v1.3.1
	github.com/minio/minio-go/v7 v7.0.6
	google.golang.org/grpc v1.34.0
	google.golang.org/protobuf v1.25.0
	testgrpc.com/grpcservice v0.0.0-00010101000000-000000000000 // indirect
	testgrpc.com/mqttapi v0.0.0-00010101000000-000000000000 // indirect
)

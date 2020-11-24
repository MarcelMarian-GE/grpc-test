#!/bin/sh

PROJDIR=$PWD
GOPATH_SRC=~/go/src/
MQTT_MODULE=mqttapi
GRPC_MODULE=grpcservice
MQTT_PATH=$GOPATH_SRC$MQTT_MODULE
GRPC_PATH=$GOPATH_SRC$GRPC_MODULE

cd proto

protoc --go_out=. $MQTT_MODULE.proto
protoc --go_out=plugins=grpc:. $GRPC_MODULE.proto
if [ ! -d $MQTT_PATH ]
then
    echo "Creating dir $MQTT_PATH"
    mkdir $MQTT_PATH
fi
if [ ! -d $GRPC_PATH ]
then
    echo "Creating dir $GRPC_PATH"
    mkdir $GRPC_PATH
fi
cp $MQTT_MODULE.pb.go $MQTT_PATH
cp $GRPC_MODULE.pb.go $GRPC_PATH

cd $PROJDIR

go build testRecv.go
go build testSend.go
go build mqttRecv.go

chmod +x testRecv
chmod +x testSend
chmod +x mqttRecv

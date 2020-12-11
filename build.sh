#!/bin/sh

PROJDIR=$PWD
PROTO_BASE=example.com
PROJ_NAME=grpctest
MQTT_MODULE=mqttapi
GRPC_MODULE=grpcservice
GRPC_SRV=testGrpcServer
GRPC_CLI=testGrpcClient
MQTT_APP=mqttApp
TEST_FILE_PUT="tmp/server/mynodered1.tar"
TEST_FILE_GET="tmp/client/mynodered.tar"

# Show the script usage
usage() {
  echo "Usage:"
  echo "    ./build.sh --target={build | clean}"
  exit
}

# Cleanup the build
clean () {
  echo "Cleanup..."
  rm -rf *.pb.*
  rm -rf proto/*.pb.*
  rm -rf $PROTO_BASE/
  rm -rf $GRPC_SRV
  rm -rf $GRPC_CLI
  rm -rf $MQTT_APP
  rm -rf $TEST_FILE_PUT
  rm -rf $TEST_FILE_GET
  rm -rf *.tar
  rm -rf *.tar.gz
  rm -rf *.zip
}


# Arguments parsing
parse_args () {
  for i in $@
  do
  case $i in
      -t=*|--target=*)
      ARG_TARGET="${i#*=}"
      shift # past argument=value
      ;;
      *)
         # unknown option
          usage
      ;;
  esac
  done
}

process_cmd () {
  case $ARG_TARGET in
      build)
      build_cmd
      ;;
      clean)
      clean
      ;;
      *)
      usage
      ;;
  esac
}

build_cmd () {
    echo "Compiling the proto files"
    protoc --go_out=. $MQTT_MODULE.proto
    protoc --go_out=plugins=grpc:. $GRPC_MODULE.proto

    cd $PROTO_BASE/$GRPC_MODULE
    go mod init "$PROTO_BASE/$GRPC_MODULE"
    cd $PROJDIR
    cd $PROTO_BASE/$MQTT_MODULE
    go mod init "$PROTO_BASE/$MQTT_MODULE"
    cd $PROJDIR
    go mod init "$PROTO_BASE/$PROJ_NAME"
    # go build
    # sed -e 's|{"go 1.13"}|{"go 1.14"}|g' go.mod > go.mod1

    CGO_ENABLED=0  go build -ldflags="-extldflags=-static" $GRPC_SRV.go
    CGO_ENABLED=0  go build -ldflags="-extldflags=-static" $GRPC_CLI.go
    CGO_ENABLED=0  go build -ldflags="-extldflags=-static" $MQTT_APP.go

    chmod +x $GRPC_SRV
    chmod +x $GRPC_CLI
    chmod +x $MQTT_APP
}

echo "GOPATH = $GOROOT"

export GO111MODULE=on
parse_args "$@"
process_cmd
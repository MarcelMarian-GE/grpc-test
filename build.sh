#!/bin/sh

PROJDIR=$PWD
GOPATH_SRC=~/go/src/
MQTT_MODULE=mqttapi
GRPC_MODULE=grpcservice
MQTT_PATH=$GOPATH_SRC$MQTT_MODULE
GRPC_PATH=$GOPATH_SRC$GRPC_MODULE
GRPC_SRV=testGrpcServer
GRPC_CLI=testGrpcClient

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
  rm -rf $GRPC_SRV
  rm -rf $GRPC_CLI
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

    go build $GRPC_SRV.go
    go build $GRPC_CLI.go
    go build mqttRecv.go

    chmod +x $GRPC_SRV
    chmod +x $GRPC_CLI
    chmod +x mqttRecv
}

parse_args "$@"
process_cmd
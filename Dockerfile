FROM golang:1.14-rc-alpine3.11 as builder

ARG APPDIR=src/grpctest

RUN apk update && apk add git protoc gcc libc-dev

# Install mqtt
RUN go get github.com/eclipse/paho.mqtt.golang
# Install protobuf compiler packages
RUN go get github.com/golang/protobuf/protoc-gen-go
RUN go get -u google.golang.org/grpc
RUN go get golang.org/x/net/context
RUN go get github.com/aws/aws-sdk-go

#RUN cp bin/protoc-gen-go /bin/

RUN mkdir $APPDIR
ADD ./ $APPDIR
WORKDIR $APPDIR
RUN ./build.sh -t=build

FROM amd64/alpine:3.11

COPY --from=builder go/src/grpctest/testGrpcServer ./
COPY --from=builder go/src/grpctest/testGrpcClient ./
COPY --from=builder go/src/grpctest/mqttApp ./
COPY ./tmp/server/test.txt ./
WORKDIR ./

RUN chmod ug+x testGrpcServer
RUN chmod ug+x testGrpcClient
RUN chmod ug+x mqttApp

# CMD ["/testGrpcServer"]
# CMD ["/testGrpcClient"]
# CMD ["/mqttApp"]

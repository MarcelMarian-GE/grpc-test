FROM golang:1.14-rc-alpine3.11 as builder

ARG APPDIR=src/grpctest

RUN apk update && apk add git protoc gcc libc-dev openssl

RUN export GO111MODULE=on
# Install mqtt
RUN go get github.com/eclipse/paho.mqtt.golang
# Install protobuf compiler packages
RUN go get github.com/golang/protobuf/protoc-gen-go
RUN go get -u google.golang.org/grpc
RUN go get golang.org/x/net/context
RUN	GO111MODULE=on go get github.com/minio/minio-go/v7
RUN	GO111MODULE=on go get github.com/minio/minio-go/v7/pkg/credentials

RUN mkdir $APPDIR
ADD ./ $APPDIR
WORKDIR $APPDIR
RUN ./build.sh -t=build

# Generate a self signed certificate and key (service)
RUN openssl genrsa -out ca.key 4096 \
	&& openssl req -new -x509 -key ca.key -sha256 -subj "/C=US/ST=NJ/O=CA, Inc." -days 365 -out ca.cert \
	&& openssl genrsa -out service.key 4096 \
	&& openssl req -new -key service.key -out service.csr -config certificate.conf \
	&& openssl x509 -req -in service.csr -CA ca.cert -CAkey ca.key -CAcreateserial \
    		-out service.pem -days 365 -sha256 -extfile certificate.conf -extensions req_ext
RUN echo "Security items"
RUN ls -l service.*

FROM amd64/alpine:3.11

COPY --from=builder go/src/grpctest/testGrpcServer ./
COPY --from=builder go/src/grpctest/testGrpcClient ./
COPY --from=builder go/src/grpctest/mqttApp ./
COPY --from=builder go/src/grpctest/service.pem /
COPY --from=builder go/src/grpctest/service.key /


# COPY ./tmp/server/test.txt ./
# COPY ./tmp/server/mynodered.tar ./
WORKDIR ./

RUN chmod ug+x testGrpcServer
RUN chmod ug+x testGrpcClient
RUN chmod ug+x mqttApp

# CMD ["/testGrpcServer"]
# CMD ["/testGrpcClient"]
# CMD ["/mqttApp"]

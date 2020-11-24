FROM golang:1.14-rc-alpine3.11 as builder

ARG APPDIR=/go/src/grpctest

RUN apk update && apk add git protoc gcc libc-dev

RUN mkdir $APPDIR
ADD . $APPDIR

# Install mqtt
RUN go get github.com/eclipse/paho.mqtt.golang

# Install protobuf go compiler packages
RUN go get github.com/golang/protobuf/protoc-gen-go
RUN go get -u google.golang.org/grpc
RUN go get golang.org/x/net/context
RUN cp /go/bin/protoc-gen-go /bin/
ADD ./ /go/src/
WORKDIR $APPDIR
RUN ./build.sh

FROM amd64/alpine:3.11
# FROM golang:1.14-rc-alpine3.11

# RUN apk update && apk add git protoc gcc libc-dev

# Install mqtt
# RUN go get github.com/eclipse/paho.mqtt.golang

# Install protobuf go packages
# RUN go get github.com/golang/protobuf/protoc-gen-go
# RUN go get -u google.golang.org/grpc
# RUN go get golang.org/x/net/context

COPY --from=builder /go/src/grpctest/testSend /
COPY --from=builder /go/src/grpctest/testRecv /
WORKDIR /

RUN chmod +x /testRecv
RUN chmod +x /testSend

# CMD ["/testSend"]
# CMD ["/testRecv"]

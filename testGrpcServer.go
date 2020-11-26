package main

import (
	context "context"
	"fmt"
	grpcSrv "grpcservice"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"google.golang.org/grpc"
)

type GrpcServiceServer struct {
}

var wg = new(sync.WaitGroup)
var client mqtt.Client

func initSignalHandle() {
	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		// Run Cleanup
		fmt.Println("Receive: get exit signal, exit now.")
		os.Exit(1)
	}()
}

func (s *GrpcServiceServer) Start(ctx context.Context, req *grpcSrv.StartRequest) (*grpcSrv.StartResponse, error) {
	log.Println("Start request processing: ", req.Message)
	client.Publish("testTopic/1", 0, false, "Testing MQTT: Start")
	return &grpcSrv.StartResponse{Result: "Start"}, nil
}

func (s *GrpcServiceServer) Stop(ctx context.Context, req *grpcSrv.StopRequest) (*grpcSrv.StopResponse, error) {
	log.Println("Stop request processing: ", req.Id)
	client.Publish("testTopic/1", 0, false, "Testing MQTT: Stop")
	return &grpcSrv.StopResponse{Result: "Stop"}, nil
}

func (s *GrpcServiceServer) TransferFile(stream grpcSrv.GrpcService_TransferFileServer) error {
	log.Println("TransferFile request processing")
	var chunkCount int32

	client.Publish("testTopic/1", 0, false, "Testing MQTT: TransferFile")
	return stream.SendAndClose(&grpcSrv.FileProgress{
		Filename:        "TestFileName",
		BytesTransfered: 100,
	})
	startTime := time.Now()
	for {
		chunk, err := stream.Recv()
		log.Println("TransferFile request processing: ", chunk.Filename)
		if err == io.EOF {
			endTime := time.Now()
			return stream.SendAndClose(&grpcSrv.FileProgress{
				Filename:        "TestFileName",
				BytesTransfered: int32(endTime.Sub(startTime).Seconds()),
			})
		}
		if err != nil {
			return err
		}
		chunkCount++
	}
	return nil
}

func mqttConnect(clientId string, uri *url.URL) mqtt.Client {
	opts := mqttCreateClientOptions(clientId, uri)
	client := mqtt.NewClient(opts)
	token := client.Connect()
	for !token.WaitTimeout(3 * time.Second) {
	}
	if err := token.Error(); err != nil {
		log.Fatal(err)
	}
	return client
}

func mqttCreateClientOptions(clientId string, uri *url.URL) *mqtt.ClientOptions {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s", uri.Host))
	opts.SetClientID(clientId)
	return opts
}

func main() {
	initSignalHandle()
	fmt.Println("gRPC server")

	uri, err := url.Parse("mqtt://172.18.0.2:1883/testTopic")
	client = mqttConnect("pub", uri)

	client.Publish("testTopic/1", 0, false, "Testing MQTT")

	// gRPC server initialization
	listener, err := net.Listen("tcp", "localhost:50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	server := GrpcServiceServer{}
	opts := []grpc.ServerOption{}
	grpcServer := grpc.NewServer(opts...)
	grpcSrv.RegisterGrpcServiceServer(grpcServer, &server)
	fmt.Println("gRPC listener")
	grpcServer.Serve(listener)

	fmt.Printf("Terminating\n")
}

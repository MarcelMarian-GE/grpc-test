package main

import (
	context "context"
	"fmt"
	grpcInterface "grpcservice"
	grpcSrv "grpcservice"
	"log"
	"net"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"google.golang.org/grpc/reflection"

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

func (s *GrpcServiceServer) Start(ctx context.Context, req *grpcInterface.StartRequest) (*grpcInterface.StartResponse, error) {
	log.Println("Start request processing: ", req.Message)
	client.Publish("testTopic/1", 0, false, "Testing MQTT: Start")
	return &grpcInterface.StartResponse{Result: "Start"}, nil
}

func (s *GrpcServiceServer) Stop(ctx context.Context, req *grpcInterface.StopRequest) (*grpcInterface.StopResponse, error) {
	log.Println("Stop request processing: ", req.Id)
	client.Publish("testTopic/1", 0, false, "Testing MQTT: Stop")
	return &grpcInterface.StopResponse{Result: "Stop"}, nil
}

func (s *GrpcServiceServer) PutFile(stream grpcInterface.GrpcService_PutFileServer) error {
	var resp grpcInterface.FileProgress
	var fileLen int32

	// Forward the notification over mqtt
	client.Publish("testTopic/1", 0, false, "Testing MQTT: PutFile")

	// Receive the file content and append it to a slice
	fileLen = 0
	for {
		chunk, err := stream.Recv()
		if err != nil {
			log.Println("Transfer File Error")
			return err
		}
		fileLen += chunk.ByteCount
		if chunk.More == false {
			resp.Filename = chunk.Filename
			resp.BytesTransfered = fileLen
			log.Println("PutFile request processing: filename =", resp.Filename, "#bytes =", resp.BytesTransfered)
			return stream.SendAndClose(&resp)
		}
	}
}

func (s *GrpcServiceServer) GetFile(req *grpcInterface.FileProgress, stream grpcInterface.GrpcService_GetFileServer) error {
	var fileChunks []*grpcSrv.FileChunk

	// Simulate the file reading
	for i := 0; i < 10; i++ {
		var chunk grpcSrv.FileChunk
		chunk.Filename = req.Filename
		chunk.Packet = []byte("Here is another string....")
		if i < 9 {
			chunk.ByteCount = int32(len(chunk.Packet))
			chunk.More = true
		} else {
			chunk.ByteCount = 11
			chunk.More = false
		}
		fileChunks = append(fileChunks, &chunk)
	}
	// Simulate the file sending
	fileLen := 0
	for _, chunk := range fileChunks {
		if err := stream.Send(chunk); err != nil {
			log.Fatalf("%v.Send(%v) = %v", stream, chunk, err)
		}
		fileLen += int(chunk.ByteCount)
	}
	log.Printf("GetFile: filename=\"%s\", bytes = %d", req.Filename, fileLen)
	return nil
}

func startGrpcServer() {
	listener, err := net.Listen("tcp", "localhost:50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	server := GrpcServiceServer{}
	opts := []grpc.ServerOption{}
	grpcServer := grpc.NewServer(opts...)
	grpcInterface.RegisterGrpcServiceServer(grpcServer, &server)

	reflection.Register(grpcServer)

	fmt.Println("gRPC listener")
	grpcServer.Serve(listener)
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
	fmt.Println("gRPC server application")

	uri, err := url.Parse("mqtt://172.18.0.2:1883/testTopic")
	if err != nil {
		log.Fatalf("failed to parse url: %v", err)
	}
	client = mqttConnect("pub", uri)

	startGrpcServer()
}

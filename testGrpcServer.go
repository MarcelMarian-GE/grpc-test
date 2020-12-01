package main

import (
	"bufio"
	context "context"
	"fmt"

	grpcInterface "grpcservice"
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

func check(e error) {
	if e != nil {
		log.Println(e)
	}
}

func (s *GrpcServiceServer) Start(ctx context.Context, req *grpcInterface.StartRequest) (*grpcInterface.StartResponse, error) {
	log.Println("Start request processing: ", req.Message)
	// Forward notification to mqtt
	client.Publish("testTopic/1", 0, false, "Testing MQTT: Start")
	return &grpcInterface.StartResponse{Result: "Start"}, nil
}

func (s *GrpcServiceServer) Stop(ctx context.Context, req *grpcInterface.StopRequest) (*grpcInterface.StopResponse, error) {
	log.Println("Stop request processing: ", req.Id)
	// Forward notification to mqtt
	client.Publish("testTopic/1", 0, false, "Testing MQTT: Stop")
	return &grpcInterface.StopResponse{Result: "Stop"}, nil
}

func (s *GrpcServiceServer) PutFile(stream grpcInterface.GrpcService_PutFileServer) error {
	var resp grpcInterface.FileProgress
	var fileLen int32
	var f *os.File
	var fileOpened bool = false

	// Prepare a local file to save it
	for {
		chunk, err := stream.Recv()
		if err != nil {
			log.Println("GetFile Error")
			break
		}
		resp.Filename = chunk.Filename
		if !fileOpened {
			f, err = os.OpenFile(resp.Filename, os.O_CREATE|os.O_RDWR, 0644)
			fileOpened = true
		}
		// Save to local file
		if chunk.ByteCount != 0 {
			chunk.Packet = chunk.Packet[:chunk.ByteCount]
			if _, err := f.Write(chunk.Packet); err != nil {
				log.Fatal(err)
			}
		} else {
			// Close the file and exit
			if err = f.Close(); err != nil {
				log.Println(err)
			}
			log.Println("Closing file:", resp.Filename, fileLen)
			break
		}
		fileLen += chunk.ByteCount
	}
	resp.BytesTransfered = fileLen
	log.Println("PutFile request: Filename =", resp.Filename, "#bytes =", resp.BytesTransfered)
	// Forward notification to mqtt
	client.Publish("testTopic/1", 0, false, "Testing MQTT: PutFile")

	return stream.SendAndClose(&resp)
}

func (s *GrpcServiceServer) GetFile(req *grpcInterface.FileProgress, stream grpcInterface.GrpcService_GetFileServer) error {
	fileLen := 0
	// Check if file exists
	f, err := os.Open(req.Filename)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer func() {
		if err = f.Close(); err != nil {
			log.Println(err)
		}
	}()
	// Read from the local file
	r := bufio.NewReader(f)
	b := make([]byte, 256)
	for {
		var chunk grpcInterface.FileChunk
		chunk.Filename = req.Filename
		n, err := r.Read(b)
		if err != nil {
			break
		} else {
			chunk.Packet = b[:n]
			fileLen += n
			chunk.ByteCount = int32(n)
			if err := stream.Send(&chunk); err != nil {
				log.Fatalf("%v.Send(%v) = %v", stream, chunk, err)
				break
			}
		}
	}

	log.Printf("GetFile: filename=\"%s\", bytes = %d", req.Filename, fileLen)
	// Forward the notification over mqtt
	client.Publish("testTopic/1", 0, false, "Testing MQTT: GetFile")
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

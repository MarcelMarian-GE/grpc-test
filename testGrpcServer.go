package main

import (
	"bufio"
	context "context"
	"flag"
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
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	// "github.com/aws/aws-sdk-go/aws"
	// "github.com/aws/aws-sdk-go/aws/awserr"
	// "github.com/aws/aws-sdk-go/aws/request"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
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
			log.Println("GetFile:", err)
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
	// Close the file once done with it
	defer func() {
		if err = f.Close(); err != nil {
			log.Println(err)
		}
	}()
	// Read chunks from the local file and stream these over gRPC
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
	listener, err := net.Listen("tcp", "grpc-server-app:50051")
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

func initializeMinio(bucket, key string, timeout time.Duration) error {
	flag.Parse()
	sess := session.Must(session.NewSession())
	svc := s3.New(sess)
	if svc == nil {
		log.Fatalf("Failed to create the session for minio")
	}
	// Create a context with a timeout that will abort the upload if it takes
	// more than the passed in timeout.
	ctx := context.Background()
	// Ensure the context is canceled to prevent leaking.
	// See context package for more information, https://golang.org/pkg/context/
	defer func() {
		if timeout > 0 {
			ctx, _ = context.WithTimeout(ctx, timeout)
		}
	}()
	return nil
}

func fileUpload() {
	// // Uploads the object to S3. The Context will interrupt the request if the
	// // timeout expires.
	// _, err := svc.PutObjectWithContext(ctx, &s3.PutObjectInput{
	// 	Bucket: aws.String(bucket),
	// 	Key:    aws.String(key),
	// 	Body:   os.Stdin,
	// })
	// if err != nil {
	// 	if aerr, ok := err.(awserr.Error); ok && aerr.Code() == request.CanceledErrorCode {
	// 		// If the SDK can determine the request or retry delay was canceled
	// 		// by a context the CanceledErrorCode error code will be returned.
	// 		fmt.Fprintf(os.Stderr, "upload canceled due to timeout, %v\n", err)
	// 	} else {
	// 		fmt.Fprintf(os.Stderr, "failed to upload object, %v\n", err)
	// 	}
	// 	os.Exit(1)
	// }

	// fmt.Printf("successfully uploaded file to %s/%s\n", bucket, key)
}

func main() {
	var err error
	var bucket, key string
	var timeout time.Duration

	flag.StringVar(&bucket, "b", "", "Bucket name.")
	flag.StringVar(&key, "k", "", "Object key name.")
	flag.DurationVar(&timeout, "d", 0, "Upload timeout.")
	flag.Parse()

	initSignalHandle()
	fmt.Println("gRPC server application")

	err = initializeMinio(bucket, key, timeout)
	// // Initialize s3 interface
	// sess := session.Must(session.NewSession())
	// svc := s3.New(sess)
	// if svc == nil {
	// 	log.Fatalf("Failed to create the session for minio")
	// }
	// // Create a context with a timeout that will abort the upload if it takes
	// // more than the passed in timeout.
	// ctx := context.Background()
	// // Ensure the context is canceled to prevent leaking.
	// // See context package for more information, https://golang.org/pkg/context/
	// defer func() {
	// 	if timeout > 0 {
	// 		ctx, _ = context.WithTimeout(ctx, timeout)
	// 	}
	// }()

	uri, err := url.Parse("mqtt://predix-edge-broker:1883/testTopic")
	if err != nil {
		log.Fatalf("failed to parse url: %v", err)
	}
	client = mqttConnect("pub", uri)

	startGrpcServer()
}

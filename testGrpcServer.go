package main

import (
	"bufio"
	context "context"
	"flag"
	"fmt"
	"io"

	"log"
	"net"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	grpcCredentials "google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
	proto "google.golang.org/protobuf/proto"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	grpcInterface "testgrpc.com/grpcservice"
	mqttInterface "testgrpc.com/mqttapi"

	"github.com/minio/minio-go/v7"
	minioCredentials "github.com/minio/minio-go/v7/pkg/credentials"
)

var _ = proto.Marshal
var _ = mqttInterface.GenericReqMsg{}

type GrpcServiceServer struct {
}

var mqttClient mqtt.Client
var endpoint = "sc-minio:9000"
var accessKeyID = "minio"

// var secretAccessKey = "cFV^+7/85KFr-ws]"
var secretAccessKey = "minio123"
var useSSL = false
var minioClient *minio.Client

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

var seqNo int32 = 0

func (s *GrpcServiceServer) StartApp(ctx context.Context, req *grpcInterface.StartAppRequest) (*grpcInterface.StartAppResponse, error) {
	// Test protobuf marshaling
	startAppMsg := &mqttInterface.StartAppMsg{
		AppName: "MyAppName",
	}
	params, _ := proto.Marshal(startAppMsg)

	mqttGenReq := &mqttInterface.GenericReqMsg{
		Opcode: mqttInterface.EnumOpcode_START_APP,
		Seqno:  seqNo,
		Params: params,
	}
	data, err := proto.Marshal(mqttGenReq)
	if err != nil {
		log.Println(err)
	}

	// Forward the notification over mqtt
	token := mqttClient.Publish("testTopic/1", 0, false, data)
	token.Wait()

	seqNo++
	log.Println("StartApp request processing: ", req.Message)

	return &grpcInterface.StartAppResponse{Result: "StartApp"}, nil
}

func (s *GrpcServiceServer) StopApp(ctx context.Context, req *grpcInterface.StopAppRequest) (*grpcInterface.StopAppResponse, error) {
	// Test protobuf marshaling
	stopAppMsg := &mqttInterface.StopAppMsg{
		AppName: "MyAppName",
	}
	params, _ := proto.Marshal(stopAppMsg)

	mqttGenReq := &mqttInterface.GenericReqMsg{
		Opcode: mqttInterface.EnumOpcode_STOP_APP,
		Seqno:  seqNo,
		Params: params,
	}
	data, err := proto.Marshal(mqttGenReq)
	if err != nil {
		log.Println(err)
	}

	// Forward the notification over mqtt
	token := mqttClient.Publish("testTopic/1", 0, false, data)
	token.Wait()

	seqNo++
	log.Println("StopApp request processing: ", req.Id)

	return &grpcInterface.StopAppResponse{Result: "StopApp"}, nil
}

func (s *GrpcServiceServer) DeployApp(stream grpcInterface.GrpcService_DeployAppServer) error {
	var resp grpcInterface.AppProgress
	var fileLen int32
	var f *os.File
	var fileOpened bool = false

	// Prepare a local file to save it
	t0 := time.Now()
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if err != io.EOF {
				log.Println("DeployApp:", err)
			}
			break
		}
		resp.Appname = chunk.Appname
		resp.Filename = chunk.Filename
		if !fileOpened {
			f, err = os.OpenFile(resp.Filename, os.O_CREATE|os.O_RDWR, 0644)
			fileOpened = true
		}
		// Save the app image to a local file
		if chunk.ByteCount != 0 {
			chunk.Packet = chunk.Packet[:chunk.ByteCount]
			if _, err := f.Write(chunk.Packet); err != nil {
				log.Println(err)
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
	t1 := time.Now()
	log.Printf("DeployApp: appname=\"%s\" filename=\"%s\", bytes = %d, took %v\n", resp.Appname, resp.Filename, fileLen, t1.Sub(t0))

	putObject("data1", resp.Appname, resp.Filename)
	resp.BytesTransfered = fileLen

	deployAppMsg := &mqttInterface.DeployAppMsg{
		AppName:  resp.Appname,
		FileName: resp.Filename,
	}
	params, _ := proto.Marshal(deployAppMsg)

	// Forward the notification over mqtt
	mqttGenReq := &mqttInterface.GenericReqMsg{
		Opcode: mqttInterface.EnumOpcode_DEPLOY_APP,
		Seqno:  seqNo,
		Params: params,
	}
	data, err := proto.Marshal(mqttGenReq)
	if err != nil {
		log.Println(err)
	}
	token := mqttClient.Publish("testTopic/1", 0, false, data)
	token.Wait()

	seqNo++

	return stream.SendAndClose(&resp)
}

func (s *GrpcServiceServer) PutFile(stream grpcInterface.GrpcService_PutFileServer) error {
	var resp grpcInterface.FileProgress
	var fileLen int32
	var f *os.File
	var fileOpened bool = false

	// Prepare a local file to save it
	t0 := time.Now()
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if err != io.EOF {
				log.Println("PutFile:", err)
			}
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
				log.Println(err)
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
	t1 := time.Now()
	log.Printf("PutFile: filename=\"%s\", bytes = %d, took %v\n", resp.Filename, fileLen, t1.Sub(t0))

	putObject("data1", resp.Filename, resp.Filename)
	resp.BytesTransfered = fileLen

	// Forward the notification over mqtt
	mqttGenReq := &mqttInterface.GenericReqMsg{
		Opcode: mqttInterface.EnumOpcode_PUTFILE,
		Seqno:  seqNo,
		// Params: params,
	}
	data, err := proto.Marshal(mqttGenReq)
	if err != nil {
		log.Println(err)
	}
	token := mqttClient.Publish("testTopic/1", 0, false, data)
	token.Wait()

	seqNo++

	return stream.SendAndClose(&resp)
}

func (s *GrpcServiceServer) GetFile(req *grpcInterface.FileProgress, stream grpcInterface.GrpcService_GetFileServer) error {
	fileLen := 0
	// Check if file exists
	getObject("data1", req.Filename, req.Filename)
	f, err := os.Open(req.Filename)
	if err != nil {
		fmt.Println("GetFile:", err)
		return nil
	}
	// Close the file once done with it
	defer func() {
		if err = f.Close(); err != nil {
			fmt.Println("GetFile:", err)
		}
	}()
	// Read chunks from the local file and stream these over gRPC
	r := bufio.NewReader(f)
	b := make([]byte, 1024)
	t0 := time.Now()
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
	t1 := time.Now()
	log.Printf("GetFile: filename=\"%s\", bytes = %d, took %v\n", req.Filename, fileLen, t1.Sub(t0))

	// Forward the notification over mqtt
	mqttGenReq := &mqttInterface.GenericReqMsg{
		Opcode: mqttInterface.EnumOpcode_GETFILE,
		Seqno:  seqNo,
	}
	data, err := proto.Marshal(mqttGenReq)
	if err != nil {
		log.Println(err)
	}
	token := mqttClient.Publish("testTopic/1", 0, false, data)
	token.Wait()

	seqNo++

	return nil
}

func startGrpcServer(sec int) {
	listener, err := net.Listen("tcp", "grpc-server-app:50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	server := GrpcServiceServer{}
	opts := []grpc.ServerOption{}

	if sec != 0 {
		// Use the self signed certificate and key for server initialization
		creds, err := grpcCredentials.NewServerTLSFromFile("service.pem", "service.key")
		if err != nil {
			log.Fatalf("failed to setup TLS with local files (cert and key)", "error", err)
			return
		}
		opts = append(opts, grpc.Creds(creds))
	}

	grpcServer := grpc.NewServer(opts...)
	grpcInterface.RegisterGrpcServiceServer(grpcServer, &server)

	reflection.Register(grpcServer)

	fmt.Println("gRPC listener")
	grpcServer.Serve(listener)
}

func mqttConnect(clientId string, mqttUrl string) (mqtt.Client, error) {
	uri, err := url.Parse(mqttUrl)
	if err != nil {
		log.Println("failed to parse url: %v", err)
	}
	opts := mqttCreateClientOptions(clientId, uri)
	client := mqtt.NewClient(opts)
	token := client.Connect()
	for !token.WaitTimeout(3 * time.Second) {
	}
	if err := token.Error(); err != nil {
		log.Println(err)
	}
	return client, err
}

func mqttCreateClientOptions(clientId string, uri *url.URL) *mqtt.ClientOptions {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s", uri.Host))
	opts.SetClientID(clientId)
	return opts
}

func initMinioClient() error {
	var err error = nil
	// Initialize minio client object.
	minioClient, err = minio.New(endpoint, &minio.Options{
		Creds:  minioCredentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	return err
}

func makeBucket(bucketName string) error {
	ctx := context.Background()
	location := "default"

	err := minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: location})
	if err != nil {
		// Check to see if there is this bucket
		exists, errBucketExists := minioClient.BucketExists(ctx, bucketName)
		if errBucketExists == nil && exists {
			fmt.Printf("We already own %s\n", bucketName)
		} else {
			fmt.Println(err)
		}
	} else {
		fmt.Printf("Successfully created %s\n", bucketName)
	}

	return err

}

func putObject(bucketName, objectName, filePath string) error {
	contentType := "application/text"

	t0 := time.Now()
	// Upload the file with FPutObject
	_, err := minioClient.FPutObject(context.Background(), bucketName, objectName, filePath, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		fmt.Printf("FPutObject ERROR (%v): bucket= %s, file = %s, path = %s\n", err, bucketName, objectName, filePath)
		return err
	}
	t1 := time.Now()
	fmt.Printf("FPutObject: Successfully stored %s in %v\n", objectName, t1.Sub(t0))

	return err
}

func getObject(bucketName, objectName, filePath string) error {
	t0 := time.Now()
	// Upload the file with FGetObject
	err := minioClient.FGetObject(context.Background(), bucketName, objectName, filePath, minio.GetObjectOptions{})
	if err != nil {
		fmt.Printf("FGetObject ERROR (%v): bucket= %s, file = %s, path = %s\n", err, bucketName, objectName, filePath)
		return err
	}
	t1 := time.Now()
	fmt.Printf("FGetObject: Successfully retrieved %s in %v\n", objectName, t1.Sub(t0))

	return err
}

func main() {
	var bucket, key, mqttUrl string
	var minioTout time.Duration
	var err error = nil

	// Initialize some local variables
	security := flag.Int("sec", 1, "Security flag.")
	flag.StringVar(&bucket, "data", "", "Bucket name.")
	flag.StringVar(&key, "key", "", "Object key name.")
	flag.DurationVar(&minioTout, "duration", 0, "Upload timeout.")
	flag.StringVar(&mqttUrl, "mqttUrl", "mqtt://docker-edge-broker:1883/testTopic", "MQTT broker URL")
	flag.Parse()

	initSignalHandle()
	fmt.Println("gRPC server application")

	// Initialize minio client
	for {
		err = initMinioClient()
		if err != nil {
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}
	makeBucket("data1")

	// Initialize mqtt client
	for {
		mqttClient, err = mqttConnect("pub", mqttUrl)
		if err != nil {
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}
	// Initialize gRPC interface
	startGrpcServer(*security)
}

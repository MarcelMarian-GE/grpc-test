package main

import (
	"bufio"
	context "context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	grpcInterface "testgrpc.com/grpcservice"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func startGrpcClient(sec int) (*grpc.ClientConn, error) {
	var opts grpc.DialOption
	if sec == 0 {
		// No security
		opts = grpc.WithInsecure()
		log.Printf("Connected to the server app NO SECURITY\n")
	} else {
		// Self signed cetificate based security
		creds, err := credentials.NewClientTLSFromFile("service.pem", "")
		if err != nil {
			log.Println(err)
		}
		opts = grpc.WithTransportCredentials(creds)
		log.Printf("Connected to the server app with a self signed certificate\n")
	}

	conn, err := grpc.Dial("grpc-server-app:50051", opts)
	if err != nil {
		log.Println(err)
	}
	return conn, err
}

func shutdownCli(conn *grpc.ClientConn) {
	if err := conn.Close(); err != nil {
		log.Println(err)
	}
}

func runGrpcDeployApp(client grpcInterface.GrpcServiceClient, appName string, srcFilename string, destFilename string) {
	// Check if file exists
	f, err := os.Open(srcFilename)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() {
		if err = f.Close(); err != nil {
			log.Println(err)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	stream, err := client.DeployApp(ctx)
	if err != nil {
		log.Fatalf("%v.DeployApp(_) = _, %v", client, err)
	}
	// File reading and streaming
	r := bufio.NewReader(f)
	b := make([]byte, 1024)
	fileLen := 0
	t0 := time.Now()
	for {
		var chunk grpcInterface.AppChunk
		chunk.Appname = appName
		chunk.Filename = destFilename
		n, err := r.Read(b)
		if err != nil {
			break
		} else {
			chunk.Packet = b[:n]
			chunk.ByteCount = int32(n)
			if err := stream.Send(&chunk); err != nil {
				log.Fatalf("%v.Send(%v) = %v", stream, chunk, err)
				break
			}
			fileLen += n
		}
	}
	t1 := time.Now()
	log.Printf("Response: DeployApp - appname=\"%s\", filename=\"%s\", len = %d, took %v", appName, destFilename, fileLen, t1.Sub(t0))

	reply, err := stream.CloseAndRecv()
	if err != nil {
		log.Printf("%v.CloseAndRecv() got error %v, want %v\n", stream, err, nil)
		return
	}
	reply.Filename = destFilename
	reply.BytesTransfered = int32(fileLen)
}

func runGrpcFilePut(client grpcInterface.GrpcServiceClient, srcFilename string, destFilename string) {
	// Check if file exists
	f, err := os.Open(srcFilename)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() {
		if err = f.Close(); err != nil {
			log.Println(err)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	stream, err := client.PutFile(ctx)
	if err != nil {
		log.Fatalf("%v.PutFile(_) = _, %v", client, err)
	}
	// File reading and streaming
	r := bufio.NewReader(f)
	b := make([]byte, 1024)
	fileLen := 0
	t0 := time.Now()
	for {
		var chunk grpcInterface.FileChunk
		chunk.Filename = destFilename
		n, err := r.Read(b)
		if err != nil {
			break
		} else {
			chunk.Packet = b[:n]
			chunk.ByteCount = int32(n)
			if err := stream.Send(&chunk); err != nil {
				log.Fatalf("%v.Send(%v) = %v", stream, chunk, err)
				break
			}
			fileLen += n
		}
	}
	t1 := time.Now()
	log.Printf("Response: FilePut - filename=\"%s\", len = %d, took %v", destFilename, fileLen, t1.Sub(t0))

	reply, err := stream.CloseAndRecv()
	if err != nil {
		log.Printf("%v.CloseAndRecv() got error %v, want %v\n", stream, err, nil)
		return
	}
	reply.Filename = destFilename
	reply.BytesTransfered = int32(fileLen)
}

func runGrpcFileGet(client grpcInterface.GrpcServiceClient, srcFilename string, destFilename string) {
	var req grpcInterface.FileProgress

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	req.Filename = srcFilename
	stream, err := client.GetFile(ctx, &req)
	if err != nil {
		log.Fatalf("%v.GetFile(_) = _, %v", client, err)
	}

	fileLen := 0
	// Prepare a local file to save it
	f, err := os.OpenFile(destFilename, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		log.Println(err)
		return
	}
	defer func() {
		if err = f.Close(); err != nil {
			log.Println(err)
		}
	}()
	t0 := time.Now()
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if err != io.EOF {
				log.Println("GetFile:", err)
			}
			break
		}
		// Save to local file
		if chunk.ByteCount != 0 {
			chunk.Packet = chunk.Packet[:chunk.ByteCount]
			if _, err := f.Write(chunk.Packet); err != nil {
				log.Println(err)
			}
		} else {
			break
		}
		fileLen += int(chunk.ByteCount)
	}
	t1 := time.Now()
	log.Printf("GetFile request: srcFilename = \"%s\", len = %d, took = %v\n", srcFilename, fileLen, t1.Sub(t0))
}

func createTestFile(destFilename string, size int) {
	f, err := os.Create(destFilename)
	defer f.Close()

	if err != nil {
		log.Println(err)
		return
	}
	for i := 0; i < size; i++ {
		buff := []byte("The quick brown fox jumps over the lazy dog.\n")
		if _, err := f.Write(buff); err != nil {
			log.Println(err)
		}
	}
}

func initSignalHandle() {
	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		// Run Cleanup
		fmt.Println("Send: get exit signal, exit now.")
		os.Exit(1)
	}()
}

func main() {
	// Initialize some local variables
	security := flag.Int("sec", 1, "Security flag.")
	testFilename := "test.txt"
	remoteFilename := "test.log"
	var conn *grpc.ClientConn = nil
	var err error = nil

	fmt.Println("Starting gRPC client client application")
	initSignalHandle()

	time.Sleep(5 * time.Second)
	// gRPC client initialization
	for {
		conn, err = startGrpcClient(*security)
		if err != nil {
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}
	// Client shutdown
	defer shutdownCli(conn)

	startAppRequest := grpcInterface.StartAppRequest{Message: "StartApp!"}
	stopAppRequest := grpcInterface.StopAppRequest{Id: "StopApp!"}
	cli := grpcInterface.NewGrpcServiceClient(conn)
	var j int32 = 0
	// Create a test file
	createTestFile(testFilename, 1000000)
	// gRPC client testing loop
	for {
		switch j % 5 {
		case 0:
			// Sending StartApp command
			startResp, err := cli.StartApp(context.Background(), &startAppRequest)
			if err != nil {
				log.Fatalf("Error when calling StartApp function: %s", err)
			}
			log.Printf("Response: %s", startResp.Result)
		case 1:
			// Sending StopApp command
			stopResp, err := cli.StopApp(context.Background(), &stopAppRequest)
			if err != nil {
				log.Fatalf("Error when calling StopApp function: %s", err)
			}
			log.Printf("Response: %s", stopResp.Result)
		case 2:
			runGrpcDeployApp(cli, "MyAppName", testFilename, remoteFilename)
		case 3:
			runGrpcFilePut(cli, testFilename, remoteFilename)
		case 4:
			runGrpcFileGet(cli, remoteFilename, remoteFilename)
		default:
			log.Printf("Unknown command")

		}
		j = j + 1
		time.Sleep(1 * time.Second)
	}

}

package main

import (
	context "context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	grpcSrv "grpcservice"

	"google.golang.org/grpc"
)

func runGrpcFilePut(client grpcSrv.GrpcServiceClient, filename string) {
	var chunksCount int = 10
	var fileChunks []*grpcSrv.FileChunk
	arr := []byte("Here is a string....")

	// Create a random number of random points
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := client.PutFile(ctx)
	if err != nil {
		log.Fatalf("%v.PutFile(_) = _, %v", client, err)
	}

	// Simulate the file reading
	for i := 0; i < chunksCount; i++ {
		var req grpcSrv.FileChunk
		req.Filename = filename
		req.Packet = arr
		if i < 9 {
			req.ByteCount = 999
			req.More = true
		} else {
			req.ByteCount = 9
			req.More = false
		}
		fileChunks = append(fileChunks, &req)
	}
	// Simulate the file sending
	for _, chunk := range fileChunks {
		if err := stream.Send(chunk); err != nil {
			log.Fatalf("%v.Send(%v) = %v", stream, chunk, err)
		}
	}
	reply, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("%v.CloseAndRecv() got error %v, want %v", stream, err, nil)
	}
	log.Printf("FilePut: filename=\"%s\", bytes = %d", reply.Filename, reply.BytesTransfered)
}

func runGrpcFileGet(client grpcSrv.GrpcServiceClient, filename string) {
	var req grpcSrv.FileProgress
	var fileChunks []*grpcSrv.FileChunk

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req.Filename = filename
	stream, err := client.GetFile(ctx, &req)
	if err != nil {
		log.Fatalf("%v.GetFile(_) = _, %v", client, err)
	}

	fileLen := 0
	for {
		chunk, err := stream.Recv()
		if err != nil {
			log.Println("GetFile Error")
			break
		}
		fileChunks = append(fileChunks, chunk)
		fileLen += int(chunk.ByteCount)
		if chunk.More == false {
			break
		}
	}
	req.BytesTransfered = int32(fileLen)
	log.Println("GetFile request: filename =", req.Filename, "#bytes =", req.BytesTransfered)
	// fmt.Printf("len=%d cap=%d %v\n", len(fileChunks), cap(fileChunks), fileChunks)
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
	fmt.Println("gRPC client client application")
	initSignalHandle()

	// gRPC client initialization
	opts := grpc.WithInsecure()
	conn, err := grpc.Dial("localhost:50051", opts)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	startRequest := grpcSrv.StartRequest{Message: "Start!"}
	stopRequest := grpcSrv.StopRequest{Id: "Stop!"}
	cli := grpcSrv.NewGrpcServiceClient(conn)
	var j int32 = 0
	// gRPC client testing loop
	for {
		if j%2 == 0 {
			// Sending Start command
			startResp, err := cli.Start(context.Background(), &startRequest)
			if err != nil {
				log.Fatalf("Error when calling Start function: %s", err)
			}
			log.Printf("Response: %s", startResp.Result)
			runGrpcFilePut(cli, "TestFileNamePut")
		} else {
			// Sending Stop command
			stopResp, err := cli.Stop(context.Background(), &stopRequest)
			if err != nil {
				log.Fatalf("Error when calling Start function: %s", err)
			}
			log.Printf("Response: %s", stopResp.Result)
			runGrpcFileGet(cli, "TestFileNameGet")
		}
		j = j + 1
		time.Sleep(1 * time.Second)
	}

}

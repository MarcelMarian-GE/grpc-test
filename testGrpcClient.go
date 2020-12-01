package main

import (
	"bufio"
	context "context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	grpcInterface "grpcservice"

	"google.golang.org/grpc"
)

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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := client.PutFile(ctx)
	if err != nil {
		log.Fatalf("%v.PutFile(_) = _, %v", client, err)
	}
	// File reading and streaming
	r := bufio.NewReader(f)
	b := make([]byte, 256)
	fileLen := 0
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
	reply, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("%v.CloseAndRecv() got error %v, want %v", stream, err, nil)
	}
	reply.Filename = destFilename
	reply.BytesTransfered = int32(fileLen)
	log.Printf("Response: FilePut - filename=\"%s\", bytes = %d", reply.Filename, reply.BytesTransfered)
}

func runGrpcFileGet(client grpcInterface.GrpcServiceClient, srcFilename string, destFilename string) {
	var req grpcInterface.FileProgress

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
		log.Println("Closing file:", destFilename, fileLen)
	}()
	for {
		chunk, err := stream.Recv()
		if err != nil {
			log.Println("GetFile Error")
			break
		}
		// Save to local file
		if chunk.ByteCount != 0 {
			chunk.Packet = chunk.Packet[:chunk.ByteCount]
			if _, err := f.Write(chunk.Packet); err != nil {
				log.Fatal(err)
			}
		} else {
			break
		}
		fileLen += int(chunk.ByteCount)
	}
	log.Println("GetFile request: srcFilename =", srcFilename, "#bytes =", fileLen)
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
	startRequest := grpcInterface.StartRequest{Message: "Start!"}
	stopRequest := grpcInterface.StopRequest{Id: "Stop!"}
	cli := grpcInterface.NewGrpcServiceClient(conn)
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
			runGrpcFilePut(cli, "tmp/client/client.log", "tmp/server/test.log")
		} else {
			// Sending Stop command
			stopResp, err := cli.Stop(context.Background(), &stopRequest)
			if err != nil {
				log.Fatalf("Error when calling Start function: %s", err)
			}
			log.Printf("Response: %s", stopResp.Result)
			runGrpcFileGet(cli, "tmp/server/test.txt", "tmp/client/client.log")
		}
		j = j + 1
		time.Sleep(1 * time.Second)
	}

}

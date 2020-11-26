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

func runGrpcFileTransfer(client grpcSrv.GrpcServiceClient) {
	var chunksCount int = 10
	var fileChunks []*grpcSrv.FileChunk
	var arr []byte
	arr = []byte("Here is a string....")

	// Create a random number of random points
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := client.TransferFile(ctx)
	if err != nil {
		log.Fatalf("%v.TransferFile(_) = _, %v", client, err)
	}

	for i := 0; i < chunksCount; i++ {
		req := grpcSrv.FileChunk{
			Filename:  "TestFilename",
			Packet:    arr,
			ByteCount: 10,
			More:      true,
		}
		fileChunks = append(fileChunks, &req)
	}
	for _, chunk := range fileChunks {
		if err := stream.Send(chunk); err != nil {
			log.Fatalf("%v.Send(%v) = %v", stream, chunk, err)
		}
	}
	reply, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("%v.CloseAndRecv() got error %v, want %v", stream, err, nil)
	}
	log.Printf("Route summary: %v", reply)
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
	initSignalHandle()

	// gRPC client initialization
	fmt.Println("gRPC Client client ...")
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
	for {
		// gRPC client testing loop
		runGrpcFileTransfer(cli)
		if j%2 == 0 {
			// Sending Start command
			startResp, err := cli.Start(context.Background(), &startRequest)
			if err != nil {
				log.Fatalf("Error when calling Start function: %s", err)
			}
			log.Printf("Response: %s", startResp.Result)
		} else {
			// Sending Stop command
			stopResp, err := cli.Stop(context.Background(), &stopRequest)
			if err != nil {
				log.Fatalf("Error when calling Start function: %s", err)
			}
			log.Printf("Response: %s", stopResp.Result)
		}
		j = j + 1
		time.Sleep(1 * time.Second)
	}

}

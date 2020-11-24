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

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"google.golang.org/grpc"
)

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

func Publisher(client mqtt.Client, topic string, payload []byte) {
	token := client.Publish(topic, 0, false, payload)
	token.Wait()
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

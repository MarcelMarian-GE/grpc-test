package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	proto "google.golang.org/protobuf/proto"
	mqttInterface "testgrpc.com/mqttapi"
)

var _ = proto.Unmarshal
var _ = mqttInterface.GenericReqMsg{}

func initSignalHandle() {
	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		// Run Cleanup
		log.Println("Send: get exit signal, exit now.")
		os.Exit(1)
	}()
}

var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	data := msg.Payload()
	genericRecvMsg := &mqttInterface.GenericReqMsg{}
	err := proto.Unmarshal(data, genericRecvMsg)
	if err != nil {
		log.Println("Unmarshaling ERROR: ", err, data)
	} else {
		switch genericRecvMsg.Opcode {
		case mqttInterface.EnumOpcode_START:
			startRecvMsg := &mqttInterface.StartMsg{}
			proto.Unmarshal(genericRecvMsg.Params, startRecvMsg)
			if err != nil {
				log.Println("Unmarshaling ERROR: ", err, data)
			} else {
				fmt.Println("Message:", genericRecvMsg.Opcode, genericRecvMsg.Seqno, startRecvMsg.Startparam1, startRecvMsg.Startparam2)
			}
		case mqttInterface.EnumOpcode_STOP:
			stopRecvMsg := &mqttInterface.StopMsg{}
			proto.Unmarshal(genericRecvMsg.Params, stopRecvMsg)
			if err != nil {
				log.Println("Unmarshaling ERROR: ", err, data)
			} else {
				fmt.Println("Message:", genericRecvMsg.Opcode, genericRecvMsg.Seqno, stopRecvMsg.StopParam)
			}
		case mqttInterface.EnumOpcode_PUTFILE:
			fmt.Println("Message:", genericRecvMsg.Opcode, genericRecvMsg.Seqno)
		case mqttInterface.EnumOpcode_GETFILE:
			fmt.Println("Message:", genericRecvMsg.Opcode, genericRecvMsg.Seqno)
		}
		// fmt.Println("Message:", genericRecvMsg.Opcode, genericRecvMsg.Seqno, genericRecvMsg.Params)
	}

	token := client.Publish("testTopic/2", 0, false, "mqtt response")
	token.Wait()
}

func main() {
	initSignalHandle()

	opts := mqtt.NewClientOptions().AddBroker("tcp://predix-edge-broker:1883")
	opts.SetClientID("mac-go")
	opts.SetDefaultPublishHandler(f)
	topic := "testTopic/1"

	opts.OnConnect = func(c mqtt.Client) {
		if token := c.Subscribe(topic, 0, f); token.Wait() && token.Error() != nil {
			log.Println(token.Error())
		}
	}
	client := mqtt.NewClient(opts)
	// Try to connect to the broker
	for {
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			time.Sleep(1 * time.Second)
		} else {
			log.Printf("Connected to broker\n")
			break
		}
	}
	for {
		time.Sleep(2000 * time.Millisecond)
	}
}

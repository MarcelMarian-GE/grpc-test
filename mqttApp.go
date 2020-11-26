package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

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
	log.Printf("MSG: %s\n", msg.Payload())
	// text := log.Sprintf("this is result msg #%d!", knt)
	token := client.Publish("testTopic/2", 0, false, "mqtt response")
	token.Wait()
}

func main() {
	initSignalHandle()

	opts := mqtt.NewClientOptions().AddBroker("tcp://172.18.0.2:1883")
	opts.SetClientID("mac-go")
	opts.SetDefaultPublishHandler(f)
	topic := "testTopic/1"

	opts.OnConnect = func(c mqtt.Client) {
		if token := c.Subscribe(topic, 0, f); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
	}
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	} else {
		log.Printf("Connected to server\n")
	}
	for {
		time.Sleep(2000 * time.Millisecond)
	}
}

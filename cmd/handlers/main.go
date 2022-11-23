package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.dfds.cloud/aad-aws-sync/internal/azuretest"
	"go.dfds.cloud/aad-aws-sync/internal/handlers"
	"go.dfds.cloud/aad-aws-sync/internal/kafkamsgs"
	"go.dfds.cloud/aad-aws-sync/internal/kafkautil"

	"github.com/kelseyhightower/envconfig"
)

func main() {
	// Process configurations
	var authConfig kafkautil.AuthSASLPlainConfig
	err := envconfig.Process("sasl_plain", &authConfig)
	if err != nil {
		log.Fatal("failed to process auth configurations:", err)
	}

	var consumerConfig kafkautil.ConsumerConfig
	err = envconfig.Process("consumer", &consumerConfig)
	if err != nil {
		log.Fatal("failed to process consumer configurations:", err)
	}

	var producerConfig kafkautil.ProducerConfig
	err = envconfig.Process("producer", &producerConfig)
	if err != nil {
		log.Fatal("failed to process producer configurations:", err)
	}

	// Iniate the dialer
	dialer := kafkautil.NewDialer(authConfig)

	// Initiate consumer
	consumer := kafkautil.NewConsumer(consumerConfig, dialer)

	// Initiate producer
	producer := kafkautil.NewProducer(producerConfig, dialer)

	// Hand clean up, need to signal to the consumer group that the
	// process is exiting so that it does not have to wait for a timeout
	// to rebalance.
	cleanup := func() {
		if err := consumer.Close(); err != nil {
			log.Fatal("failed to close kafka reader:", err)
		}
	}
	defer cleanup()

	// Clean up on SIGINT and SIGTERM.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Println("caught signal", sig)
		cleanup()
	}()

	// Mock the Azure Client
	azureClient := &azuretest.MockAzureClient{
		CreateGroupMock: func(name string) (string, error) { return "mock-aad67fcd-5a26-4e1a-98aa-bd6d4eb828ac", nil },
	}

	// Initiate handlers context
	ctx := context.WithValue(context.Background(), handlers.ContextKeyAzureClient, azureClient)
	ctx = context.WithValue(ctx, handlers.ContextKeyKafkaProducer, producer)

	// Read messages
	for {
		log.Println("fetch messages")
		m, err := consumer.FetchMessage(ctx)
		if err == io.EOF {
			log.Println("connection closed")
			break
		} else if err != nil {
			log.Fatal("error fetching message:", err)
		}
		fmt.Printf("message at topic/partition/offset %v/%v/%v: %s = %s\n", m.Topic, m.Partition, m.Offset, string(m.Key), string(m.Value))
		log.Println("message headers", m.Headers)

		var msg kafkamsgs.CapabilityCreatedMessage
		err = json.Unmarshal(m.Value, &msg)
		if err != nil {
			// TODO forward the message to the dead letter queue
			log.Fatal("error unmarshaling message", err)
		}

		log.Printf("processed capability created message: %s", msg)

		// Handle the message
		err = handlers.CapabilityCreatedHandler(ctx, msg)
		if err != nil {
			log.Fatal("error handling capability created message", err)
		}

		log.Println("dump create group calls on mock:", azureClient.CreateGroupCalls)

		// Commit offset to the consumer group
		if err := consumer.CommitMessages(ctx, m); err != nil {
			log.Fatal("failed to commit messages:", err)
		}
	}
}

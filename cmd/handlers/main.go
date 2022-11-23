package main

import (
	"context"
	"encoding/json"
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
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/protocol"
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

	var errorProducerConfig kafkautil.ProducerConfig
	err = envconfig.Process("error_producer", &errorProducerConfig)
	if err != nil {
		log.Fatal("failed to process error producer configurations:", err)
	}

	// Iniate the dialer
	dialer := kafkautil.NewDialer(authConfig)

	// Initiate consumer
	consumer := kafkautil.NewConsumer(consumerConfig, dialer)

	// Initiate producers
	producer := kafkautil.NewProducer(producerConfig, dialer)
	defer producer.Close()
	errorProducer := kafkautil.NewProducer(errorProducerConfig, dialer)
	defer errorProducer.Close()

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
		log.Printf("message at topic/partition/offset %v/%v/%v: %s = %s : %v\n", m.Topic, m.Partition, m.Offset, string(m.Key), string(m.Value), m.Headers)

		var msg kafkamsgs.CapabilityCreatedMessage
		err = json.Unmarshal(m.Value, &msg)
		if err != nil {
			// Write the message along with the error to the dead letter queue.
			err = errorProducer.WriteMessages(ctx, kafka.Message{
				Key:   m.Key,
				Value: m.Value,
				Headers: append(m.Headers, protocol.Header{
					Key:   kafkamsgs.HeaderKeyError,
					Value: []byte(err.Error()),
				}),
			})
			if err != nil {
				log.Fatal("error writing message to dead letter queue", err)
			}
			log.Println("error unmarshaling message, message with error written to dead letter queue", err)
			continue
		}

		log.Printf("processed capability created message: %s", msg)

		// Handle the message
		err = handlers.CapabilityCreatedHandler(ctx, msg)
		if err != nil {
			// TODO should these errors be reported to dead letter queue? yes, if not temporary
			log.Fatal("error handling capability created message", err)
		}

		log.Println("dump create group calls on mock:", azureClient.CreateGroupCalls)

		// Commit offset to the consumer group
		if err := consumer.CommitMessages(ctx, m); err != nil {
			log.Fatal("failed to commit messages:", err)
		}
	}
}

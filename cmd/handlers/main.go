package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"go.dfds.cloud/aad-aws-sync/internal/azure"
	"go.dfds.cloud/aad-aws-sync/internal/azuretest"
	"go.dfds.cloud/aad-aws-sync/internal/handlers"
	"go.dfds.cloud/aad-aws-sync/internal/kafkamsgs"
	"go.dfds.cloud/aad-aws-sync/internal/kafkautil"

	"github.com/kelseyhightower/envconfig"
	"github.com/stretchr/testify/mock"
)

type Config struct {
	AzureParentAdministrativeUnitID string `envconfig:"azure_parent_administrative_unit_id",required:"true"`
}

func main() {
	// Process configurations
	var config Config
	err := envconfig.Process("main", &config)
	if err != nil {
		log.Fatal("failed to process main configurations:", err)
	}

	var authConfig kafkautil.AuthConfig
	err = envconfig.Process("sasl_plain", &authConfig)
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
	consumer := kafkautil.NewConsumer(consumerConfig, authConfig, dialer)

	// Initiate producers
	producer := kafkautil.NewProducer(producerConfig, authConfig, dialer)
	defer producer.Close()
	// Write errors to a dead letter queue
	errorProducer := kafkautil.NewProducer(errorProducerConfig, authConfig, dialer)
	defer errorProducer.Close()

	// Hand clean up, need to signal to the consumer group that the
	// process is exiting so that it does not have to wait for a timeout
	// to rebalance.
	var cleanupOnce sync.Once
	cleanup := func() {
		log.Println("closing consumer")
		if err := consumer.Close(); err != nil {
			log.Fatal("failed to close kafka reader:", err)
		}
		log.Println("consumer closed")
	}
	defer cleanupOnce.Do(cleanup)

	// Mock the Azure Client
	azureClient := new(azuretest.MockAzureClient)
	azureClient.On("CreateAdministrativeUnitGroup", mock.Anything, mock.Anything).Return(&azure.CreateAdministrativeUnitGroupResponse{ID: "mock-aad67fcd-5a26-4e1a-98aa-bd6d4eb828ac"}, nil)

	// Initiate handlers context
	ctx, cancel := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, handlers.ContextKeyAzureClient, azureClient)
	ctx = context.WithValue(ctx, handlers.ContextKeyAzureParentAdministrativeUnitID, config.AzureParentAdministrativeUnitID)
	ctx = context.WithValue(ctx, handlers.ContextKeyKafkaProducer, producer)
	ctx = context.WithValue(ctx, handlers.ContextKeyKafkaErrorProducer, errorProducer)

	// Clean up on SIGINT and SIGTERM.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Println("caught signal", sig)
		cancel()
		log.Println("canceled pending requests")
		cleanupOnce.Do(cleanup)
	}()

	// Read messages from the topic
	for {
		log.Println("fetch messages")
		m, err := consumer.FetchMessage(ctx)
		if err == io.EOF {
			log.Println("connection closed")
			break
		} else if err == context.Canceled {
			log.Println("processing canceled")
			break
		} else if err != nil {
			log.Fatal("error fetching message:", err)
		}
		log.Printf("message at topic/partition/offset %v/%v/%v: %s = %s : %v\n", m.Topic, m.Partition, m.Offset, string(m.Key), string(m.Value), m.Headers)

		// Parse event metadata from the message to determine if we are expected to handle it
		event := kafkamsgs.NewEventFromMessage(m)
		if event == nil || event.Name == "" {
			// Handle undetermined event name
			handlers.PermanentErrorHandler(ctx, *event, errors.New("unable to detect an event name"))
			goto CommitOffset
		}

		// Route the event to the appropriate handler based on event name and version
		switch event.Name {
		case kafkamsgs.EventNameCapabilityCreated:
			switch event.Version {
			case kafkamsgs.Version1:
				// Handle the event
				handlers.CapabilityCreatedHandler(ctx, *event)
			default:
				handlers.PermanentErrorHandler(ctx, *event, fmt.Errorf("unsupported version of the capability created event: %q\n", event.Version))
			}
		default:
			// Skip unhandled event
			log.Printf("skip processing unhandled event: %q\n", event.Name)
		}

		// Commit offset to the consumer group
	CommitOffset:
		if err := consumer.CommitMessages(ctx, m); err != nil {
			log.Fatal("failed to commit offset:", err)
		}
	}
}

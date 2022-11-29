package main

import (
	"context"
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
	"go.dfds.cloud/aad-aws-sync/internal/router"

	"github.com/kelseyhightower/envconfig"
	"github.com/stretchr/testify/mock"
)

type Config struct {
	AzureParentAdministrativeUnitID string `envconfig:"azure_parent_administrative_unit_id",required:"true"`
}

// eventHandlers gathers a set of event handlers to meet the router's
// EventHandlers interface.
type eventHandlers struct{}

func (_ eventHandlers) PermanentErrorHandler(ctx context.Context, event kafkamsgs.Event, err error) {
	handlers.PermanentErrorHandler(ctx, event, err)
}

func (_ eventHandlers) CapabilityCreatedHandler(ctx context.Context, event kafkamsgs.Event) {
	handlers.CapabilityCreatedHandler(ctx, event)
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

	// Initiate producers
	producer := kafkautil.NewProducer(producerConfig, authConfig, dialer)
	defer producer.Close()
	// Write errors to a dead letter queue
	errorProducer := kafkautil.NewProducer(errorProducerConfig, authConfig, dialer)
	defer errorProducer.Close()

	// Mock the Azure Client
	// TODO(emil): need to replace this with the actual client
	azureClient := new(azuretest.MockAzureClient)
	azureClient.On("CreateAdministrativeUnitGroup", mock.Anything, mock.Anything).
		Return(&azure.CreateAdministrativeUnitGroupResponse{ID: "mock-aad67fcd-5a26-4e1a-98aa-bd6d4eb828ac"}, nil)

	// Initiate handlers context
	ctx, cancel := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, handlers.ContextKeyAzureClient, azureClient)
	ctx = context.WithValue(ctx, handlers.ContextKeyAzureParentAdministrativeUnitID, config.AzureParentAdministrativeUnitID)
	ctx = context.WithValue(ctx, handlers.ContextKeyKafkaProducer, producer)
	ctx = context.WithValue(ctx, handlers.ContextKeyKafkaErrorProducer, errorProducer)
	ctx = context.WithValue(ctx, router.ContextKeyKafkaConsumer, consumer)
	ctx = context.WithValue(ctx, router.ContextKeyEventHandlers, eventHandlers{})

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

	// Start consuming messages
	router.ConsumeMessages(ctx)
}

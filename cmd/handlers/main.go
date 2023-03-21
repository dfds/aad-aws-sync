package main

import (
	"context"
	"go.dfds.cloud/aad-aws-sync/internal/event_handlers"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"go.dfds.cloud/aad-aws-sync/internal/azure"
	"go.dfds.cloud/aad-aws-sync/internal/azuretest"
	"go.dfds.cloud/aad-aws-sync/internal/kafkamsgs"
	"go.dfds.cloud/aad-aws-sync/internal/kafkautil"
	"go.dfds.cloud/aad-aws-sync/internal/router"

	"github.com/kelseyhightower/envconfig"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

const (
	DevelopmentEnvironment string = "development"
	ProductionEnvironment         = "production"
)

type Config struct {
	Environment                     string `default:"development"`
	AzureParentAdministrativeUnitID string `envconfig:"azure_parent_administrative_unit_id",required:"true"`
}

// eventHandlers gathers a set of event handlers to meet the router's
// EventHandlers interface.
type eventHandlers struct{}

func (_ eventHandlers) PermanentErrorHandler(ctx context.Context, event kafkamsgs.Event, err error) {
	event_handlers.PermanentErrorHandler(ctx, event, err)
}

func (_ eventHandlers) CapabilityCreatedHandler(ctx context.Context, event kafkamsgs.Event) {
	event_handlers.CapabilityCreatedHandler(ctx, event)
}

func main() {
	// Initiate the logger
	log := zap.Must(zap.NewDevelopment())

	// Process configurations
	var config Config
	err := envconfig.Process("main", &config)
	if err != nil {
		log.Fatal("Failed to process main configurations", zap.Error(err))
	}

	// Switch logging to configured environment
	switch config.Environment {
	case ProductionEnvironment:
		log = zap.Must(zap.NewProduction())
	case DevelopmentEnvironment:
		// no op
	default:
		log.Fatal("Unexpected environment specified",
			zap.String("environment", config.Environment))
	}

	var authConfig kafkautil.AuthConfig
	err = envconfig.Process("sasl_plain", &authConfig)
	if err != nil {
		log.Fatal("Failed to process auth configurations", zap.Error(err))
	}

	var consumerConfig kafkautil.ConsumerConfig
	err = envconfig.Process("consumer", &consumerConfig)
	if err != nil {
		log.Fatal("Failed to process consumer configurations", zap.Error(err))
	}

	var producerConfig kafkautil.ProducerConfig
	err = envconfig.Process("producer", &producerConfig)
	if err != nil {
		log.Fatal("Failed to process producer configurations", zap.Error(err))
	}

	var errorProducerConfig kafkautil.ProducerConfig
	err = envconfig.Process("error_producer", &errorProducerConfig)
	if err != nil {
		log.Fatal("Failed to process error producer configurations", zap.Error(err))
	}

	// Iniate the dialer
	dialer, _ := kafkautil.NewDialer(authConfig)

	// Initiate consumer
	consumer := kafkautil.NewConsumer(consumerConfig, authConfig, dialer)
	// Hand clean up, need to signal to the consumer group that the
	// process is exiting so that it does not have to wait for a timeout
	// to rebalance.
	var cleanupOnce sync.Once
	cleanup := func() {
		log.Debug("Closing Kafka consumer")
		if err := consumer.Close(); err != nil {
			log.Fatal("Failed to close Kafka consumer", zap.Error(err))
		}
		log.Debug("Kafka consumer has been closed")
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
	ctx = context.WithValue(ctx, event_handlers.ContextKeyAzureClient, azureClient)
	ctx = context.WithValue(ctx, event_handlers.ContextKeyAzureParentAdministrativeUnitID, config.AzureParentAdministrativeUnitID)
	ctx = context.WithValue(ctx, event_handlers.ContextKeyKafkaProducer, producer)
	ctx = context.WithValue(ctx, event_handlers.ContextKeyKafkaErrorProducer, errorProducer)
	ctx = context.WithValue(ctx, router.ContextKeyKafkaConsumer, consumer)
	ctx = context.WithValue(ctx, router.ContextKeyEventHandlers, eventHandlers{})
	ctx = context.WithValue(ctx, router.ContextKeyLogger, log)

	// Clean up on SIGINT and SIGTERM.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Debug("Caught signal", zap.String("signal", sig.String()))
		cancel()
		log.Info("Canceling pending requests")
		cleanupOnce.Do(cleanup)
	}()

	// Start consuming messages
	router.ConsumeMessages(ctx)
}

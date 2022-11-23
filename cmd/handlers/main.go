package main

// TODO organize code into packages

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
)

// KafkaConsumerConfig allows one to configure a Kafka consumer using
// environment variables.
type KafkaConsumerConfig struct {
	Brokers []string `required:"true"`
	GroupID string   `envconfig:"group_id",required:"true"`
	Topic   string   `required:"true"`
	// TODO split this out into its own config
	SASLPlainUsername string `envconfig:"sasl_plain_username",required:"true"`
	SASLPlainPassword string `envconfig:"sasl_plain_password",required:"true"`
}

// Example message published by the capability service:
// {
//   "version": "1",
//   "eventName": "capability_created",
//   "x-correlationId": "e2c2dbf6-0318-4aa2-8765-15ef75c5def3",
//   "x-sender": "CapabilityService.WebApi, Version=1.0.0.0, Culture=neutral, PublicKeyToken=null",
//   "payload": {
//     "capabilityId": "2335d768-add3-4022-89ba-3d5c2b5cdba3",
//     "capabilityName": "Sandbox-samolak"
//   }
// }

type CapabilityCreatedMessagePayload struct {
	CapabilityID   string `json:"capabilityId"`
	CapabilityName string `json:"capabilityName"`
}

type CapabilityCreatedMessage struct {
	Version        string                          `json:"version"`
	EventName      string                          `json:"eventName"`
	XCorellationID string                          `json:"x-corellationId"`
	XSender        string                          `json:"x-sender"`
	Payload        CapabilityCreatedMessagePayload `json:"payload"`
}

func (msg CapabilityCreatedMessage) String() string {
	return fmt.Sprintf("%s (%s)", msg.Payload.CapabilityName, msg.Payload.CapabilityID)
}

const (
	ContextKeyAzureClient int = iota
)

func GetAzureClient(ctx context.Context) AzureClient {
	return ctx.Value(ContextKeyAzureClient).(AzureClient)
}

// MockAzureClient is used to mock the AzureClient interface.
type MockAzureClient struct {
	createGroupCalls []string
	createGroupMock  func(string) (string, error)
}

func (c *MockAzureClient) CreateGroup(name string) (string, error) {
	c.createGroupCalls = append(c.createGroupCalls, name)
	return c.createGroupMock(name)
}

// TODO replace with real interface later, but this is approx
type AzureClient interface {
	CreateGroup(name string) (string, error)
}

func CapabilityCreatedHandler(ctx context.Context, msg CapabilityCreatedMessage) error {
	log.Println("capability created handler", msg)

	// Create an Azure AD group
	azureClient := GetAzureClient(ctx)
	groupID, err := azureClient.CreateGroup(msg.Payload.CapabilityName)
	if err != nil {
		return err
	}
	log.Println("created azure ad group:", msg.Payload.CapabilityName, groupID)

	// TODO publish an event with the azure ad group id
	return nil
}

func main() {
	// configure a kafka client
	// poll kafka topic for capability creation events
	// parse the kafka messages

	// TODO pass the message to an event handler, the message would just contain the capability id essentially
	// TODO the event handler would create azure active directory group with for the capability
	// TODO the event handler would then publish a kafka message with the capability id and the azure active directory id on another topic
	// TODO the event handler would then also commit the message offset to the consumer group

	// TODO handlers would be passed in a kafka publisher instance, which could be mocked during testing of the handler
	// TODO hanlders would also be passed a azure client instance, which could be mocked during testing of the handler

	// Process configurations
	var consumerConfig KafkaConsumerConfig
	err := envconfig.Process("consumer", &consumerConfig)
	if err != nil {
		log.Fatal("failed to process consumer configurations:", err)
	}

	// Configure TLS
	tlsConfig := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
	}

	// Configure SASL
	saslMechanism := plain.Mechanism{
		Username: consumerConfig.SASLPlainUsername,
		Password: consumerConfig.SASLPlainPassword,
	}

	// Configure connection dialer
	dialer := &kafka.Dialer{
		Timeout:       10 * time.Second,
		DualStack:     true,
		TLS:           tlsConfig,
		SASLMechanism: saslMechanism,
	}

	// Initiate reader
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: consumerConfig.Brokers,
		GroupID: consumerConfig.GroupID,
		Topic:   consumerConfig.Topic,
		Dialer:  dialer,
	})

	// Hand clean up, need to signal to the consumer group that the
	// process is exiting so that it does not have to wait for a timeout
	// to rebalance.
	cleanup := func() {
		if err := r.Close(); err != nil {
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

	// Initiate handlers context
	azureClient := &MockAzureClient{
		createGroupMock: func(name string) (string, error) { return "aad67fcd-5a26-4e1a-98aa-bd6d4eb828ac", nil },
	}
	ctx := context.WithValue(context.Background(), ContextKeyAzureClient, azureClient)

	// Read messages
	for {
		log.Println("fetch messages")
		m, err := r.FetchMessage(ctx)
		if err == io.EOF {
			log.Println("connection closed")
			break
		} else if err != nil {
			log.Fatal("error fetching message:", err)
		}
		fmt.Printf("message at topic/partition/offset %v/%v/%v: %s = %s\n", m.Topic, m.Partition, m.Offset, string(m.Key), string(m.Value))
		log.Println("message headers", m.Headers)

		// TODO in case of an error pass the message off to the dead letter queue with the error

		var msg CapabilityCreatedMessage
		err = json.Unmarshal(m.Value, &msg)
		if err != nil {
			log.Fatal("error unmarshaling message", err)
		}

		log.Printf("processed capability created message: %s", msg)

		// Handle the message
		err = CapabilityCreatedHandler(ctx, msg)
		if err != nil {
			log.Fatal("error handling capability created message", err)
		}

		log.Println("dump create group calls on mock:", azureClient.createGroupCalls)

		// Commit offset to the consumer group
		if err := r.CommitMessages(ctx, m); err != nil {
			log.Fatal("failed to commit messages:", err)
		}
	}
}

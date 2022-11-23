package main

// TODO organize code into packages

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.dfds.cloud/aad-aws-sync/internal/kafkamsgs"
	"go.dfds.cloud/aad-aws-sync/internal/kafkautil"

	"github.com/kelseyhightower/envconfig"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/protocol"
)

const (
	ContextKeyAzureClient int = iota
	ContextKeyKafkaProducer
)

func GetAzureClient(ctx context.Context) AzureClient {
	return ctx.Value(ContextKeyAzureClient).(AzureClient)
}

func GetKafkaProducer(ctx context.Context) KafkaProducer {
	return ctx.Value(ContextKeyKafkaProducer).(KafkaProducer)
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

type MockKafkaProducer struct {
	writeMessagesCalls [][]kafka.Message
	writeMessagesMock  func(context.Context, ...kafka.Message) error
}

func (p *MockKafkaProducer) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	p.writeMessagesCalls = append(p.writeMessagesCalls, msgs)
	return p.writeMessagesMock(ctx, msgs...)
}

type KafkaProducer interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
}

func CapabilityCreatedHandler(ctx context.Context, msg kafkamsgs.CapabilityCreatedMessage) error {
	log.Println("capability created handler", msg)

	azureClient := GetAzureClient(ctx)
	kafkaProducer := GetKafkaProducer(ctx)

	// Create an Azure AD group
	groupID, err := azureClient.CreateGroup(msg.Payload.CapabilityName)
	if err != nil {
		return err
	}
	log.Println("created azure ad group:", msg.Payload.CapabilityName, groupID)

	// Publish a message with the result
	resp, err := json.Marshal(&kafkamsgs.AzureADGroupCreatedMessage{
		CapabilityName: msg.Payload.CapabilityName,
		AzureADGroupID: groupID,
	})
	if err != nil {
		return err
	}

	return kafkaProducer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(groupID),
		Value: resp,
		Headers: []protocol.Header{
			{
				Key:   kafkamsgs.HeaderKeyVersion,
				Value: []byte(kafkamsgs.VersionAzureADGroupCreated),
			},
			{
				Key:   kafkamsgs.HeaderKeyEventName,
				Value: []byte(kafkamsgs.EventNameAzureADGroupCreated),
			},
		},
	})
}

func main() {
	// configure a kafka client
	// poll kafka topic for capability creation events
	// parse the kafka messages

	// pass the message to an event handler, the message would just contain the capability id essentially
	// the event handler would create azure active directory group with for the capability
	// the event handler would then publish a kafka message with the capability id and the azure active directory id on another topic
	// the event handler would then also commit the message offset to the consumer group

	// handlers would be passed in a kafka publisher instance, which could be mocked during testing of the handler
	// hanlders would also be passed a azure client instance, which could be mocked during testing of the handler

	// Process configurations
	var consumerConfig kafkautil.ConsumerConfig
	err := envconfig.Process("consumer", &consumerConfig)
	if err != nil {
		log.Fatal("failed to process consumer configurations:", err)
	}

	var producerConfig kafkautil.ProducerConfig
	err = envconfig.Process("producer", &producerConfig)
	if err != nil {
		log.Fatal("failed to process producer configurations:", err)
	}

	var authConfig kafkautil.AuthSASLPlainConfig
	err = envconfig.Process("sasl_plain", &producerConfig)
	if err != nil {
		log.Fatal("failed to process auth configurations:", err)
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

	// Initiate handlers context

	// Configure Azure Client
	azureClient := &MockAzureClient{
		createGroupMock: func(name string) (string, error) { return "aad67fcd-5a26-4e1a-98aa-bd6d4eb828ac", nil },
	}
	ctx := context.WithValue(context.Background(), ContextKeyAzureClient, azureClient)
	ctx = context.WithValue(ctx, ContextKeyKafkaProducer, producer)

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
		err = CapabilityCreatedHandler(ctx, msg)
		if err != nil {
			log.Fatal("error handling capability created message", err)
		}

		log.Println("dump create group calls on mock:", azureClient.createGroupCalls)

		// Commit offset to the consumer group
		if err := consumer.CommitMessages(ctx, m); err != nil {
			log.Fatal("failed to commit messages:", err)
		}
	}
}

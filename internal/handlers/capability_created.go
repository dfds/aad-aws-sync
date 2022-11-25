package handlers

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/protocol"
	"go.dfds.cloud/aad-aws-sync/internal/kafkamsgs"
)

// TODO write unit tests for this

func CapabilityCreatedHandler(ctx context.Context, event kafkamsgs.Event) {
	log.Println("capability created handler:", event.Name, event.Version)

	// Parse the payload
	var msg kafkamsgs.CapabilityCreatedMessage
	err := json.Unmarshal(event.Message.Value, &msg)
	if err != nil {
		PermanentErrorHandler(ctx, event, err)
		return
	}

	// Attempt to create an Azure AD group with exponential backoff
	// or until the context is marked done.
	azureClient := GetAzureClient(ctx)
	// TODO should try to make this operation idempotent, try to find some unique constraint on these groups and use to make sure that the group is created only once
	createGroup := func() (string, error) {
		// TODO determine if the error is permanent here
		// TODO pass the context into this operation
		return azureClient.CreateGroup(msg.Payload.CapabilityName)
	}
	createGroupBackoff := backoff.WithContext(backoff.NewExponentialBackOff(), ctx)
	groupID, err := backoff.RetryNotifyWithData[string](createGroup, createGroupBackoff, func(err error, d time.Duration) {
		log.Printf("failed creating group after %s with %s", d, err)
	})
	if err != nil {
		// Hit a permanent error, send it off to the dead letter queue
		PermanentErrorHandler(ctx, event, err)
		return
	}

	// Encode message with the result
	kafkaProducer := GetKafkaProducer(ctx)
	resp, err := json.Marshal(&kafkamsgs.AzureADGroupCreatedMessage{
		CapabilityName: msg.Payload.CapabilityName,
		AzureADGroupID: groupID,
	})
	if err != nil {
		PermanentErrorHandler(ctx, event, err)
		return
	}

	// Produce message with the result
	produceResponseBackoff := backoff.WithContext(backoff.NewExponentialBackOff(), ctx)
	produceResponse := func() error {
		// TODO any permanent errors here?
		return kafkaProducer.WriteMessages(ctx, kafka.Message{
			Key:   []byte(groupID),
			Value: resp,
			Headers: []protocol.Header{
				{
					Key:   kafkamsgs.HeaderKeyVersion,
					Value: []byte(kafkamsgs.Version1),
				},
				{
					Key:   kafkamsgs.HeaderKeyEventName,
					Value: []byte(kafkamsgs.EventNameAzureADGroupCreated),
				},
			},
		})
	}
	err = backoff.RetryNotify(produceResponse, produceResponseBackoff, func(err error, d time.Duration) {
		log.Printf("failed producing create azure ad respnose after %s with %s", d, err)
	})
	if err != nil {
		PermanentErrorHandler(ctx, event, err)
		return
	}
}

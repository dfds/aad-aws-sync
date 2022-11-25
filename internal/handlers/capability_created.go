package handlers

import (
	"context"
	"encoding/json"
	"log"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/protocol"
	"go.dfds.cloud/aad-aws-sync/internal/kafkamsgs"
)

// TODO write unit tests for this
// TODO handle done context

func CapabilityCreatedHandler(ctx context.Context, event kafkamsgs.Event) {
	log.Println("capability created handler:", event.Name, event.Version)

	// Parse the payload
	var msg kafkamsgs.CapabilityCreatedMessage
	err := json.Unmarshal(event.Message.Value, &msg)
	if err != nil {
		PermanentErrorHandler(ctx, event, err)
		return
	}

	// Create an Azure AD group
	azureClient := GetAzureClient(ctx)
	// TODO should try to make this operation idempotent, try to find some unique constraint on these groups and use to make sure that the group is created only once
	groupID, err := azureClient.CreateGroup(msg.Payload.CapabilityName)
	if err != nil {
		// TODO retry with exponential backoff if temporary error, otherwise to dlq
		PermanentErrorHandler(ctx, event, err)
		return
	}
	log.Println("created azure ad group:", msg.Payload.CapabilityName, groupID)

	// Publish a message with the result
	kafkaProducer := GetKafkaProducer(ctx)
	resp, err := json.Marshal(&kafkamsgs.AzureADGroupCreatedMessage{
		CapabilityName: msg.Payload.CapabilityName,
		AzureADGroupID: groupID,
	})
	if err != nil {
		PermanentErrorHandler(ctx, event, err)
		return
	}

	// TODO retry with exponential backoff if temporary error, otherwise to dlq
	err = kafkaProducer.WriteMessages(ctx, kafka.Message{
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
	if err != nil {
		PermanentErrorHandler(ctx, event, err)
		return
	}
}

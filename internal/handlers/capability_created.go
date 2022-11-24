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
				Value: []byte(kafkamsgs.Version1),
			},
			{
				Key:   kafkamsgs.HeaderKeyEventName,
				Value: []byte(kafkamsgs.EventNameAzureADGroupCreated),
			},
		},
	})
}

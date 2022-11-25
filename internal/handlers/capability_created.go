package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/protocol"
	"go.dfds.cloud/aad-aws-sync/internal/azure"
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
	azureParentAdministrativeUnitID := GetAzureParentAdministrativeUnitID(ctx)
	// TODO should try to make this operation idempotent, try to find some unique constraint on these groups and use to make sure that the group is created only once, perhapps enabling mail would make it idempotent since the mailbox should be unique
	createGroupRequest := azure.CreateAdministrativeUnitGroupRequest{
		OdataType:       "#Microsoft.Graph.Group",
		Description:     "[Automated] - aad-aws-sync",
		DisplayName:     fmt.Sprintf("CI_SSU_Cap - %s", msg.Payload.CapabilityID),
		MailNickname:    fmt.Sprintf("ci-ssu_cap_%s", msg.Payload.CapabilityID),
		GroupTypes:      []interface{}{},
		MailEnabled:     false,
		SecurityEnabled: true,

		ParentAdministrativeUnitId: azureParentAdministrativeUnitID,
	}
	createGroup := func() (*azure.CreateAdministrativeUnitGroupResponse, error) {
		// TODO determine if the error is permanent here
		return azureClient.CreateAdministrativeUnitGroup(ctx, createGroupRequest)
	}
	createGroupBackoff := backoff.WithContext(backoff.NewExponentialBackOff(), ctx)
	azureADGroup, err := backoff.RetryNotifyWithData[*azure.CreateAdministrativeUnitGroupResponse](createGroup, createGroupBackoff, func(err error, d time.Duration) {
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
		AzureADGroupID: azureADGroup.ID,
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
			Key:   []byte(azureADGroup.ID),
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

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/segmentio/kafka-go"
	"go.dfds.cloud/aad-aws-sync/internal/azure"
	"go.dfds.cloud/aad-aws-sync/internal/kafkamsgs"
	"go.dfds.cloud/aad-aws-sync/internal/kafkautil"
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
		var apiError azure.ApiError
		group, err := azureClient.CreateAdministrativeUnitGroup(ctx, createGroupRequest)
		if errors.As(err, &apiError) && apiError.StatusCode >= 500 {
			// Retry 5XX errors.
			return nil, err
		} else if err != nil {
			// Assume all other errors are permanent.
			return nil, backoff.Permanent(err)
		}
		return group, nil
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
		err := kafkaProducer.WriteMessages(ctx, kafka.Message{
			Key:     []byte(azureADGroup.ID),
			Value:   resp,
			Headers: kafkamsgs.EventHeaders(kafkamsgs.EventNameAzureADGroupCreated, kafkamsgs.Version1),
		})
		if kafkautil.IsTemporaryError(err) || kafkautil.IsTransientNetworkError(err) {
			// Disabled the retry logic within the Kafka client as we want to indefinitely retry
			// on these type of errors and the backoff packages provides a more sophisticated retry
			// implementation.
			return err
		} else if err != nil {
			return backoff.Permanent(err)
		}
		return nil
	}
	err = backoff.RetryNotify(produceResponse, produceResponseBackoff, func(err error, d time.Duration) {
		log.Printf("failed producing create azure ad response after %s with %s", d, err)
	})
	if err != nil {
		PermanentErrorHandler(ctx, event, err)
		return
	}
}

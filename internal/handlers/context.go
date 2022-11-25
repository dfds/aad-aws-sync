package handlers

import (
	"context"

	"github.com/segmentio/kafka-go"
	"go.dfds.cloud/aad-aws-sync/internal/azure"
)

const (
	ContextKeyAzureClient int = iota
	ContextKeyAzureParentAdministrativeUnitID
	ContextKeyKafkaProducer
	ContextKeyKafkaErrorProducer
)

func GetAzureClient(ctx context.Context) AzureClient {
	return ctx.Value(ContextKeyAzureClient).(AzureClient)
}

func GetAzureParentAdministrativeUnitID(ctx context.Context) string {
	return ctx.Value(ContextKeyAzureParentAdministrativeUnitID).(string)
}

func GetKafkaProducer(ctx context.Context) KafkaProducer {
	return ctx.Value(ContextKeyKafkaProducer).(KafkaProducer)
}

func GetKafkaErrorProducer(ctx context.Context) KafkaProducer {
	return ctx.Value(ContextKeyKafkaErrorProducer).(KafkaProducer)
}

type AzureClient interface {
	CreateAdministrativeUnitGroup(ctx context.Context, requestPayload azure.CreateAdministrativeUnitGroupRequest) (*azure.CreateAdministrativeUnitGroupResponse, error)
}

type KafkaProducer interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
}

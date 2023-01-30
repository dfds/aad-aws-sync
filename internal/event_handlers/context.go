package event_handlers

import (
	"context"

	"github.com/segmentio/kafka-go"
	"go.dfds.cloud/aad-aws-sync/internal/azure"
	"go.uber.org/zap"
)

const (
	ContextKeyLogger                          string = "context_key_logger"
	ContextKeyAzureClient                            = "handlers_context_key_azure_client"
	ContextKeyAzureParentAdministrativeUnitID        = "handlers_context_key_azure_parent_administrative_unit_id"
	ContextKeyKafkaProducer                          = "handlers_context_key_kafka_producer"
	ContextKeyKafkaErrorProducer                     = "handlers_context_key_kafka_error_producer"
)

func GetLogger(ctx context.Context) *zap.Logger {
	return ctx.Value(ContextKeyLogger).(*zap.Logger)
}

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

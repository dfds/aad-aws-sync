package router

import (
	"context"

	"github.com/segmentio/kafka-go"
	"go.dfds.cloud/aad-aws-sync/internal/kafkamsgs"
	"go.uber.org/zap"
)

const (
	ContextKeyLogger        string = "context_key_logger"
	ContextKeyKafkaConsumer        = "router_context_key_kafka_consumer"
	ContextKeyEventHandlers        = "router_context_key_event_handlers"
)

func GetLogger(ctx context.Context) *zap.Logger {
	return ctx.Value(ContextKeyLogger).(*zap.Logger)
}

func GetKafkaConsumer(ctx context.Context) KafkaConsumer {
	return ctx.Value(ContextKeyKafkaConsumer).(KafkaConsumer)
}

func GetEventHandlers(ctx context.Context) EventHandlers {
	return ctx.Value(ContextKeyEventHandlers).(EventHandlers)
}

type KafkaConsumer interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
}

type EventHandlers interface {
	PermanentErrorHandler(ctx context.Context, event kafkamsgs.Event, err error)
	CapabilityCreatedHandler(ctx context.Context, event kafkamsgs.Event)
}

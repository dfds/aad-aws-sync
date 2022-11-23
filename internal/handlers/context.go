package handlers

import (
	"context"

	"github.com/segmentio/kafka-go"
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

type AzureClient interface {
	CreateGroup(name string) (string, error)
}

type KafkaProducer interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
}

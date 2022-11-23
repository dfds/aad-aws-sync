package kafkatest

import (
	"context"

	"github.com/segmentio/kafka-go"
)

// MockKafkaProducer is used to mock a Kafka producer.
type MockKafkaProducer struct {
	writeMessagesCalls [][]kafka.Message
	writeMessagesMock  func(context.Context, ...kafka.Message) error
}

func (p *MockKafkaProducer) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	p.writeMessagesCalls = append(p.writeMessagesCalls, msgs)
	return p.writeMessagesMock(ctx, msgs...)
}

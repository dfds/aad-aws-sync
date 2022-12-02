package kafkatest

import (
	"context"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/mock"
)

// MockKafkaProducer is used to mock a Kafka producer.
type MockKafkaProducer struct {
	mock.Mock
}

func (p *MockKafkaProducer) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	cargs := make([]interface{}, len(msgs)+1)
	for i := range cargs {
		if i == 0 {
			cargs[0] = ctx
			continue
		}
		cargs[i] = msgs[i-1]
	}
	args := p.Called(cargs...)
	return args.Error(0)
}

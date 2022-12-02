package kafkatest

import (
	"context"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/mock"
)

// MockKafkaConsumer is used to mock a Kafka consumer.
type MockKafkaConsumer struct {
	mock.Mock
}

func (c *MockKafkaConsumer) FetchMessage(ctx context.Context) (kafka.Message, error) {
	args := c.Called(ctx)
	if args.Get(0) == nil {
		return kafka.Message{}, args.Error(1)
	}
	return args.Get(0).(kafka.Message), args.Error(1)
}

func (c *MockKafkaConsumer) CommitMessages(ctx context.Context, msgs ...kafka.Message) error {
	cargs := make([]interface{}, len(msgs)+1)
	for i := range cargs {
		if i == 0 {
			cargs[0] = ctx
			continue
		}
		cargs[i] = msgs[i-1]
	}
	args := c.Called(cargs...)
	return args.Error(0)
}

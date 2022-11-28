package kafkautil

import (
	"time"

	"github.com/segmentio/kafka-go"
)

func NewProducer(config ProducerConfig, authConfig AuthConfig, dialer *kafka.Dialer) *kafka.Writer {
	return kafka.NewWriter(kafka.WriterConfig{
		Brokers:  authConfig.Brokers,
		Topic:    config.Topic,
		Balancer: &kafka.Hash{},
		Async:    false,
		Dialer:   dialer,
		// Not utilizing the internal retry logic of this client, since we want to keep trying
		// indefinitely on these type of errors.
		MaxAttempts:  1,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	})
}

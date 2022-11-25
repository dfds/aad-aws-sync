package kafkautil

import (
	"github.com/segmentio/kafka-go"
)

func NewProducer(config ProducerConfig, authConfig AuthConfig, dialer *kafka.Dialer) *kafka.Writer {
	return kafka.NewWriter(kafka.WriterConfig{
		Brokers:  authConfig.Brokers,
		Topic:    config.Topic,
		Balancer: &kafka.Hash{},
		Dialer:   dialer,
	})
}

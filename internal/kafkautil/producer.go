package kafkautil

import (
	"github.com/segmentio/kafka-go"
)

func NewProducer(config ProducerConfig, dialer *kafka.Dialer) *kafka.Writer {
	return kafka.NewWriter(kafka.WriterConfig{
		Brokers:  config.Brokers,
		Topic:    config.Topic,
		Balancer: &kafka.Hash{},
		Dialer:   dialer,
	})
}

package kafkautil

import (
	"github.com/segmentio/kafka-go"
)

func NewConsumer(config ConsumerConfig, authConfig AuthConfig, dialer *kafka.Dialer) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers: authConfig.Brokers,
		GroupID: config.GroupID,
		Topic:   config.Topic,
		Dialer:  dialer,
	})
}

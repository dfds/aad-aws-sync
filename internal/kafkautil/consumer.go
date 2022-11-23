package kafkautil

import (
	"github.com/segmentio/kafka-go"
)

func NewConsumer(config ConsumerConfig, dialer *kafka.Dialer) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers: config.Brokers,
		GroupID: config.GroupID,
		Topic:   config.Topic,
		Dialer:  dialer,
	})
}

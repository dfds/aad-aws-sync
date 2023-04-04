package kafkautil

import (
	"testing"
)

func TestNewConsumer(t *testing.T) {
	authConfig := AuthConfig{
		Brokers:          []string{"localhost:9092"},
		Mechanism:        "plain",
		MechanismOptions: nil,
		Tls:              false,
	}

	dialer, err := NewDialer(authConfig)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	consumer := NewConsumer(
		ConsumerConfig{Topic: "dummy", GroupID: "aadawssync"},
		authConfig,
		dialer,
	)

	if consumer == nil {
		t.Error("Instance of Writer not returned as expected")
		t.FailNow()
	}
}
